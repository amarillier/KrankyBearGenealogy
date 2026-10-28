// Package premerge manages the automatic safety backups taken before a GEDCOM
// merge: where they live, what they are called, and when they are cleaned up.
package premerge

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxAge is how long a pre-merge backup is kept before it is cleaned up.
const MaxAge = 14 * 24 * time.Hour

const marker = "-pre-merge-"

// Dir is the folder holding pre-merge backups for the database at dbPath.
func Dir(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "merge-backups")
}

// FileName is the backup file name for the database at dbPath taken at t.
func FileName(dbPath string, t time.Time) string {
	base := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	return base + marker + t.Format("20060102-150405") + ".zip"
}

// Cleanup deletes pre-merge backups in dir older than maxAge and removes the
// directory if that leaves it empty. It returns how many files were deleted.
// Files that are not pre-merge backups are never touched.
func Cleanup(dir string, maxAge time.Duration, now time.Time) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	remaining := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || !strings.Contains(e.Name(), marker) || !strings.HasSuffix(e.Name(), ".zip") {
			remaining++
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				removed++
				continue
			}
		}
		remaining++
	}
	if remaining == 0 {
		_ = os.Remove(dir)
	}
	return removed
}
