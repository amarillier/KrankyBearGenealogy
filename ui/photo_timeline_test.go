package ui

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"genealogy/store"
)

// testJPEG returns a small JPEG; if exifDate ("2001:02:03 10:00:00") is set, an
// Exif block carrying it as DateTimeOriginal is spliced in after the SOI marker.
func testJPEG(t *testing.T, exifDate string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		img.Set(x, x, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	if exifDate == "" {
		return buf.Bytes()
	}
	// TIFF (little endian): IFD0 -> Exif IFD pointer; Exif IFD -> DateTimeOriginal
	le := binary.LittleEndian
	tiff := make([]byte, 8+2+12+4+2+12+4)
	copy(tiff, "II")
	le.PutUint16(tiff[2:], 42)
	le.PutUint32(tiff[4:], 8)
	le.PutUint16(tiff[8:], 1)
	le.PutUint16(tiff[10:], 0x8769)
	le.PutUint16(tiff[12:], 4)
	le.PutUint32(tiff[14:], 1)
	le.PutUint32(tiff[18:], 26) // Exif IFD offset
	le.PutUint16(tiff[26:], 1)
	le.PutUint16(tiff[28:], 0x9003)
	le.PutUint16(tiff[30:], 2)
	le.PutUint32(tiff[32:], uint32(len(exifDate)+1))
	le.PutUint32(tiff[36:], uint32(len(tiff)))
	tiff = append(tiff, append([]byte(exifDate), 0)...)

	app1 := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(app1)+2))
	out := append([]byte{0xFF, 0xD8}, seg...)
	out = append(out, app1...)
	return append(out, buf.Bytes()[2:]...)
}

func writeFileT(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGroupMediaByYear(t *testing.T) {
	media := []store.Media{
		{ID: 1, Title: "b", DateTaken: "1995-06-01"},
		{ID: 2, Title: "z", DateTaken: ""},
		{ID: 3, Title: "a", DateTaken: "1995-01-15"},
		{ID: 4, Title: "old", DateTaken: "1901"},
		{ID: 5, Title: "textual", DateTaken: "3 DEC 1995"},
		{ID: 6, Title: "junk", DateTaken: "someday"},
	}
	groups := groupMediaByYear(media)
	var labels []string
	for _, g := range groups {
		labels = append(labels, g.Label)
	}
	want := []string{"1901", "1995", undatedLabel}
	if len(labels) != 3 || labels[0] != want[0] || labels[1] != want[1] || labels[2] != want[2] {
		t.Fatalf("labels = %v, want %v", labels, want)
	}
	var order []int64
	for _, m := range groups[1].Media {
		order = append(order, m.ID)
	}
	if len(order) != 3 || order[0] != 3 || order[1] != 1 || order[2] != 5 { // Jan 15, Jun 1, Dec 3
		t.Errorf("1995 order = %v, want [3 1 5]", order)
	}
	if len(groups[2].Media) != 2 {
		t.Errorf("undated = %d, want 2", len(groups[2].Media))
	}
}

func TestFindPhotos(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.PNG", "sub/c.jpeg", "notes.txt", ".hidden.jpg", ".thumbs/d.jpg", "movie.mp4"} {
		writeFileT(t, filepath.Join(dir, name), []byte("x"))
	}
	files, truncated, err := findPhotos(dir, 100)
	if err != nil || truncated {
		t.Fatalf("err=%v truncated=%v", err, truncated)
	}
	if len(files) != 3 {
		t.Errorf("found %v, want a.jpg, b.PNG, sub/c.jpeg", files)
	}
	files, truncated, _ = findPhotos(dir, 2)
	if len(files) != 2 || !truncated {
		t.Errorf("limit: %d files, truncated=%v", len(files), truncated)
	}
}

func TestImportPhotos(t *testing.T) {
	s := newExportTestStore(t)
	person := &store.Person{GivenName: "Anna", Surname: "Berg"}
	if err := s.CreatePerson(person); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFileT(t, filepath.Join(dir, "dated.jpg"), testJPEG(t, "2001:02:03 10:00:00"))
	writeFileT(t, filepath.Join(dir, "plain.jpg"), testJPEG(t, ""))
	writeFileT(t, filepath.Join(dir, "corrupt.jpg"), []byte("not really a jpeg"))
	files, _, _ := findPhotos(dir, 100)

	var reports int
	res := importPhotos(s, files, false, true, []int64{person.ID}, func(int, string) { reports++ })
	if res.Imported != 2 || res.Dated != 1 || res.FailedCount != 1 || res.Skipped != 0 {
		t.Fatalf("result = %+v", res)
	}
	if reports == 0 {
		t.Error("progress was never reported")
	}
	media, _ := s.GetMediaForPerson(person.ID)
	if len(media) != 2 {
		t.Fatalf("person has %d media, want 2", len(media))
	}
	dates := map[string]string{}
	for _, m := range media {
		dates[m.Title] = m.DateTaken
		if m.MediaType != "image" || m.IsExternal {
			t.Errorf("%s: type %q external %v", m.Title, m.MediaType, m.IsExternal)
		}
	}
	if dates["dated"] != "2001-02-03" || dates["plain"] != "" {
		t.Errorf("dates = %v", dates)
	}
	if thumb, err := s.GetMediaThumbnail(media[0].ID); err != nil || len(thumb) == 0 {
		t.Errorf("thumbnail missing: %v", err)
	}

	// Importing the same folder again skips what is already there
	res = importPhotos(s, files, false, true, []int64{person.ID}, func(int, string) {})
	if res.Imported != 0 || res.Skipped != 2 {
		t.Errorf("second run = %+v", res)
	}
	// ...unless skipping is turned off
	res = importPhotos(s, files, false, false, []int64{person.ID}, func(int, string) {})
	if res.Imported != 2 {
		t.Errorf("without dedupe = %+v", res)
	}
}

func TestImportPhotosExternal(t *testing.T) {
	s := newExportTestStore(t)
	person := &store.Person{GivenName: "Anna", Surname: "Berg"}
	s.CreatePerson(person)
	dir := t.TempDir()
	path := filepath.Join(dir, "ext.jpg")
	writeFileT(t, path, testJPEG(t, "1999:12:31 23:59:59"))

	res := importPhotos(s, []string{path}, true, true, []int64{person.ID}, func(int, string) {})
	if res.Imported != 1 || res.Dated != 1 {
		t.Fatalf("result = %+v", res)
	}
	media, _ := s.GetMediaForPerson(person.ID)
	if len(media) != 1 || !media[0].IsExternal || media[0].ExternalPath != path || media[0].DateTaken != "1999-12-31" {
		t.Errorf("media = %+v", media)
	}
	full, _ := s.GetMediaByID(media[0].ID)
	if len(full.FullImage) != 0 {
		t.Error("external media should not store the image in the database")
	}
}

func TestFillMissingPhotoDates(t *testing.T) {
	s := newExportTestStore(t)
	dir := t.TempDir()
	extPath := filepath.Join(dir, "ext.jpg")
	writeFileT(t, extPath, testJPEG(t, "1980:05:06 00:00:00"))

	mk := func(m *store.Media) int64 {
		t.Helper()
		if err := s.CreateMedia(m); err != nil {
			t.Fatal(err)
		}
		return m.ID
	}
	dbDated := mk(&store.Media{Title: "db", MediaType: "image", FullImage: testJPEG(t, "1970:01:02 00:00:00")})
	dbNoExif := mk(&store.Media{Title: "noexif", MediaType: "image", FullImage: testJPEG(t, "")})
	ext := mk(&store.Media{Title: "ext", MediaType: "image", IsExternal: true, ExternalPath: extPath})
	missing := mk(&store.Media{Title: "gone", MediaType: "image", IsExternal: true, ExternalPath: filepath.Join(dir, "nope.jpg")})
	already := mk(&store.Media{Title: "has", MediaType: "image", DateTaken: "1960-01-01", FullImage: testJPEG(t, "1970:01:02 00:00:00")})
	doc := mk(&store.Media{Title: "pdf", MediaType: "document", FullImage: testJPEG(t, "1970:01:02 00:00:00")})

	scanned, updated, err := fillMissingPhotoDates(s)
	if err != nil {
		t.Fatal(err)
	}
	if scanned != 4 || updated != 2 {
		t.Errorf("scanned %d updated %d, want 4 and 2", scanned, updated)
	}
	want := map[int64]string{dbDated: "1970-01-02", dbNoExif: "", ext: "1980-05-06", missing: "", already: "1960-01-01", doc: ""}
	for id, date := range want {
		m, _ := s.GetMediaByID(id)
		if m.DateTaken != date {
			t.Errorf("%s: date %q, want %q", m.Title, m.DateTaken, date)
		}
		if len(m.FullImage) == 0 && !m.IsExternal {
			t.Errorf("%s: image data was lost", m.Title)
		}
	}
}
