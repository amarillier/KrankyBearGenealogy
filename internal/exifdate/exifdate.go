// Package exifdate reads the date a photo was taken from its EXIF metadata.
// It handles JPEG files only and has no dependencies. Input is untrusted, so
// every offset and length is bounds-checked and malformed data just yields no date.
package exifdate

import (
	"encoding/binary"
	"io"
	"os"
	"strconv"
	"time"
)

const (
	tagDateTime          = 0x0132 // IFD0: file modification date
	tagExifIFD           = 0x8769 // IFD0: pointer to the Exif sub-IFD
	tagDateTimeOriginal  = 0x9003 // Exif IFD: when the photo was taken
	tagDateTimeDigitized = 0x9004 // Exif IFD: when it was digitised (scanned)

	maxHeaderBytes = 1 << 20 // the Exif block is at most 64KB and sits at the start
	maxIFDEntries  = 4096
)

// FromFile returns the date taken ("YYYY-MM-DD") of the JPEG at path.
func FromFile(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxHeaderBytes))
	if err != nil {
		return "", false
	}
	return DateTaken(data)
}

// DateTaken returns the date taken ("YYYY-MM-DD") from JPEG data, preferring
// DateTimeOriginal, then DateTimeDigitized, then DateTime.
func DateTaken(data []byte) (string, bool) {
	tiff := exifBlock(data)
	if tiff == nil {
		return "", false
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return "", false
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return "", false
	}

	ifd0 := readIFD(tiff, order, order.Uint32(tiff[4:8]))
	var exifTags map[uint16]string
	if off, ok := ifd0.pointers[tagExifIFD]; ok {
		exifTags = readIFD(tiff, order, off).dates
	}
	for _, candidate := range []string{exifTags[tagDateTimeOriginal], exifTags[tagDateTimeDigitized], ifd0.dates[tagDateTime]} {
		if d, ok := parseDate(candidate); ok {
			return d, true
		}
	}
	return "", false
}

// exifBlock finds the Exif APP1 segment in JPEG data and returns its TIFF
// payload (at least a full TIFF header long), or nil.
func exifBlock(data []byte) []byte {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return nil
		}
		marker := data[pos+1]
		if marker == 0xFF { // fill byte
			pos++
			continue
		}
		if marker == 0xD9 || marker == 0xDA { // end of image / start of scan
			return nil
		}
		segLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if segLen < 2 || pos+2+segLen > len(data) {
			return nil
		}
		payload := data[pos+4 : pos+2+segLen]
		if marker == 0xE1 && len(payload) >= 6+8 && string(payload[:6]) == "Exif\x00\x00" {
			return payload[6:]
		}
		pos += 2 + segLen
	}
	return nil
}

type ifd struct {
	dates    map[uint16]string // ASCII date tags found
	pointers map[uint16]uint32 // LONG tags found (sub-IFD offsets)
}

// readIFD reads the entries of one IFD, keeping only the tags this package needs.
func readIFD(tiff []byte, order binary.ByteOrder, offset uint32) ifd {
	out := ifd{dates: map[uint16]string{}, pointers: map[uint16]uint32{}}
	if uint64(offset)+2 > uint64(len(tiff)) {
		return out
	}
	count := int(order.Uint16(tiff[offset:]))
	if count > maxIFDEntries {
		return out
	}
	for i := 0; i < count; i++ {
		entry := uint64(offset) + 2 + uint64(i)*12
		if entry+12 > uint64(len(tiff)) {
			break
		}
		e := tiff[entry : entry+12]
		tag := order.Uint16(e[0:2])
		typ := order.Uint16(e[2:4])
		n := order.Uint32(e[4:8])
		switch {
		case tag == tagExifIFD && typ == 4: // LONG
			out.pointers[tag] = order.Uint32(e[8:12])
		case (tag == tagDateTime || tag == tagDateTimeOriginal || tag == tagDateTimeDigitized) && typ == 2 && n >= 10 && n <= 64:
			start := uint64(order.Uint32(e[8:12]))
			if start+uint64(n) > uint64(len(tiff)) {
				continue
			}
			s := tiff[start : start+uint64(n)]
			for len(s) > 0 && s[len(s)-1] == 0 {
				s = s[:len(s)-1]
			}
			out.dates[tag] = string(s)
		}
	}
	return out
}

// parseDate converts an EXIF "YYYY:MM:DD HH:MM:SS" value to "YYYY-MM-DD",
// rejecting placeholder zeros and impossible dates.
func parseDate(s string) (string, bool) {
	if len(s) < 10 || s[4] != ':' || s[7] != ':' {
		return "", false
	}
	y, err1 := strconv.Atoi(s[0:4])
	m, err2 := strconv.Atoi(s[5:7])
	d, err3 := strconv.Atoi(s[8:10])
	if err1 != nil || err2 != nil || err3 != nil || y < 1826 || y > time.Now().Year()+1 {
		return "", false
	}
	// time.Date normalises overflow (e.g. Feb 30), so compare the round trip
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return "", false
	}
	return t.Format("2006-01-02"), true
}
