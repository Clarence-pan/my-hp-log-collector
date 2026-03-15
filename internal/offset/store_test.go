package offset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_SetGetSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "offsets.json")

	s := NewStore(path)
	s.Set("/var/log/app.log", 123)
	s.Set("/var/log/other.log", 456)

	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	// 重新加载验证持久化。
	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if off, ok := s2.Get("/var/log/app.log"); !ok || off != 123 {
		t.Fatalf("unexpected offset for app.log: %d, ok=%v", off, ok)
	}
	if off, ok := s2.Get("/var/log/other.log"); !ok || off != 456 {
		t.Fatalf("unexpected offset for other.log: %d, ok=%v", off, ok)
	}

	s2.Delete("/var/log/app.log")
	if _, ok := s2.Get("/var/log/app.log"); ok {
		t.Fatalf("expected app.log to be deleted")
	}
}

