package exifdate

import (
	"encoding/binary"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

type entry struct {
	tag, typ uint16
	value    string // ASCII payload (stored out of line) for typ 2
	long     uint32 // value for typ 4
}

// buildJPEG assembles a minimal JPEG whose Exif block has the given IFD0
// entries and, if exif is non-nil, an Exif sub-IFD with those entries.
func buildJPEG(order binary.ByteOrder, ifd0, exif []entry) []byte {
	tiff := make([]byte, 8)
	if order == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)

	var exifOffset uint32
	if exif != nil {
		ifd0 = append(ifd0, entry{tag: tagExifIFD, typ: 4}) // patched below
	}
	ifdSize := 2 + 12*len(ifd0) + 4
	exifOffset = uint32(8 + ifdSize)
	exifIFDSize := 0
	if exif != nil {
		exifIFDSize = 2 + 12*len(exif) + 4
	}
	dataStart := 8 + ifdSize + exifIFDSize

	var data []byte
	write := func(entries []entry, base int) []byte {
		b := make([]byte, 2+12*len(entries)+4)
		order.PutUint16(b, uint16(len(entries)))
		for i, e := range entries {
			o := 2 + 12*i
			order.PutUint16(b[o:], e.tag)
			order.PutUint16(b[o+2:], e.typ)
			switch e.typ {
			case 2:
				payload := append([]byte(e.value), 0)
				order.PutUint32(b[o+4:], uint32(len(payload)))
				order.PutUint32(b[o+8:], uint32(dataStart+len(data)))
				data = append(data, payload...)
			case 4:
				order.PutUint32(b[o+4:], 1)
				if e.tag == tagExifIFD {
					order.PutUint32(b[o+8:], exifOffset)
				} else {
					order.PutUint32(b[o+8:], e.long)
				}
			}
		}
		return b
	}
	tiff = append(tiff, write(ifd0, 8)...)
	if exif != nil {
		tiff = append(tiff, write(exif, int(exifOffset))...)
	}
	tiff = append(tiff, data...)

	app1 := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(app1)+2))
	jpeg := []byte{0xFF, 0xD8}
	jpeg = append(jpeg, seg...)
	jpeg = append(jpeg, app1...)
	return append(jpeg, 0xFF, 0xD9)
}

func TestDateTaken(t *testing.T) {
	orig := entry{tag: tagDateTimeOriginal, typ: 2, value: "2019:07:04 14:03:22"}
	digi := entry{tag: tagDateTimeDigitized, typ: 2, value: "2001:01:02 00:00:00"}
	mod := entry{tag: tagDateTime, typ: 2, value: "2021:12:25 08:00:00"}

	for name, order := range map[string]binary.ByteOrder{"little": binary.LittleEndian, "big": binary.BigEndian} {
		cases := []struct {
			desc       string
			ifd0, exif []entry
			want       string
		}{
			{"original wins", []entry{mod}, []entry{digi, orig}, "2019-07-04"},
			{"digitized beats modified", []entry{mod}, []entry{digi}, "2001-01-02"},
			{"modified fallback", []entry{mod}, nil, "2021-12-25"},
			{"invalid original falls through", []entry{mod}, []entry{{tag: tagDateTimeOriginal, typ: 2, value: "0000:00:00 00:00:00"}}, "2021-12-25"},
			{"impossible date rejected", nil, []entry{{tag: tagDateTimeOriginal, typ: 2, value: "2019:02:30 10:00:00"}}, ""},
			{"no dates", nil, nil, ""},
		}
		for _, c := range cases {
			got, ok := DateTaken(buildJPEG(order, c.ifd0, c.exif))
			if (c.want != "") != ok || got != c.want {
				t.Errorf("%s/%s: got %q, %v; want %q", name, c.desc, got, ok, c.want)
			}
		}
	}
}

func TestFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.jpg")
	data := buildJPEG(binary.LittleEndian, nil, []entry{{tag: tagDateTimeOriginal, typ: 2, value: "1955:05:06 00:00:00"}})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := FromFile(path); !ok || got != "1955-05-06" {
		t.Errorf("FromFile = %q, %v", got, ok)
	}
	if _, ok := FromFile(filepath.Join(t.TempDir(), "missing.jpg")); ok {
		t.Error("missing file should give no date")
	}
}

func TestNonJPEGAndTruncated(t *testing.T) {
	for _, data := range [][]byte{nil, {0xFF}, []byte("\x89PNG\r\n\x1a\n"), []byte("not an image at all")} {
		if _, ok := DateTaken(data); ok {
			t.Errorf("%q should give no date", data)
		}
	}
	// Every truncation of a valid file must be handled without panicking
	full := buildJPEG(binary.BigEndian, nil, []entry{{tag: tagDateTimeOriginal, typ: 2, value: "2019:07:04 14:03:22"}})
	for i := 0; i < len(full); i++ {
		DateTaken(full[:i])
	}
}

func TestMalformedNeverPanics(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	base := buildJPEG(binary.LittleEndian, []entry{{tag: tagDateTime, typ: 2, value: "2021:12:25 08:00:00"}},
		[]entry{{tag: tagDateTimeOriginal, typ: 2, value: "2019:07:04 14:03:22"}})
	for i := 0; i < 20000; i++ {
		d := append([]byte(nil), base...)
		for j := 0; j < 1+rng.Intn(6); j++ {
			d[rng.Intn(len(d))] = byte(rng.Intn(256))
		}
		DateTaken(d)
	}
	for i := 0; i < 2000; i++ {
		d := make([]byte, rng.Intn(200))
		rng.Read(d)
		if len(d) > 2 {
			d[0], d[1] = 0xFF, 0xD8
		}
		DateTaken(d)
	}
}
