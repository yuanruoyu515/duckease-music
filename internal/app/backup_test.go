package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A backup holds the settings and the covers, and restoring it puts the
// covers back as they were, dropping those made since.
func TestBackupRoundTrip(t *testing.T) {
	dir := t.TempDir()
	covers := filepath.Join(dir, "covers")
	os.MkdirAll(covers, 0o700)
	os.WriteFile(filepath.Join(covers, "7.jpg"), []byte("cover"), 0o600)
	os.WriteFile(filepath.Join(covers, "notes.txt"), []byte("not a cover"), 0o600)
	s := defaultSettings()
	s.Appearance, s.Fallback = "dark", true
	s.Sidebar = []sideItem{{ID: "mine"}, {ID: "fm", Hidden: true}}
	s.SearchHistory = []string{"晴天"}
	dest := filepath.Join(dir, "icloud", backupFile)
	if err := writeBackup(dest, s, covers, time.Unix(1000, 0)); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(covers, "8.jpg"), []byte("made since"), 0o600)
	b, err := readBackup(dest)
	if err != nil {
		t.Fatal(err)
	}
	got := b.settings
	if got.Appearance != "dark" || !got.Fallback || len(got.Sidebar) != 2 || got.Sidebar[1] != (sideItem{ID: "fm", Hidden: true}) || got.SearchHistory[0] != "晴天" {
		t.Errorf("restored settings %+v", got)
	}
	if len(b.covers) != 1 || string(b.covers["7.jpg"]) != "cover" {
		t.Errorf("restored covers %v, want 7.jpg alone", b.covers)
	}
	if err := restoreCovers(covers, b.covers); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(covers, "8.jpg")); err == nil {
		t.Error("a cover made since the backup stayed")
	}
	if _, err := os.Stat(filepath.Join(covers, "notes.txt")); err != nil {
		t.Error("restoring removed a file that is not a cover")
	}
}

// A backup not there says so, and one iCloud has not downloaded yet says
// that instead.
func TestReadMissingBackup(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, backupFile)
	if _, err := readBackup(dest); err != errNoBackup {
		t.Errorf("got %v, want errNoBackup", err)
	}
	os.WriteFile(filepath.Join(dir, "."+backupFile+".icloud"), nil, 0o600)
	if _, err := readBackup(dest); err != errNotDownloaded {
		t.Errorf("got %v, want errNotDownloaded", err)
	}
}
