package premerge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileName(t *testing.T) {
	got := FileName("/data/family.db", time.Date(2026, 10, 27, 9, 5, 3, 0, time.UTC))
	if want := "family-pre-merge-20261027-090503.zip"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCleanup(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	write := func(name string, age time.Duration) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	write("a-pre-merge-1.zip", 30*24*time.Hour)
	write("a-pre-merge-2.zip", 24*time.Hour)
	write("manual-backup.zip", 90*24*time.Hour)

	if n := Cleanup(dir, MaxAge, now); n != 1 {
		t.Errorf("removed %d, want 1", n)
	}
	for _, name := range []string{"a-pre-merge-2.zip", "manual-backup.zip"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s should remain: %v", name, err)
		}
	}
}

func TestCleanupRemovesEmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "merge-backups")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "x-pre-merge-1.zip")
	os.WriteFile(p, nil, 0o644)
	old := time.Now().Add(-60 * 24 * time.Hour)
	os.Chtimes(p, old, old)
	Cleanup(dir, MaxAge, time.Now())
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("empty directory should be removed")
	}
}
