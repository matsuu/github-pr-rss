package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicallyNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feed.rss")

	if err := writeFileAtomically(path, []byte("<?xml version=\"1.0\"?><rss/>")); err != nil {
		t.Fatalf("writeFileAtomically: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "<?xml version=\"1.0\"?><rss/>" {
		t.Errorf("content = %q", got)
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(path); err != nil {
			t.Fatalf("Stat: %v", err)
		} else if st.Mode().Perm() != 0o644 {
			t.Errorf("mode = %o, want 644", st.Mode().Perm())
		}
	}
	// no leftover temporary files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory contains %d entries, want 1: %v", len(entries), entries)
	}
}

func TestWriteFileAtomicallyOverwritePreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feed.rss")
	if err := os.WriteFile(path, []byte("old content"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := writeFileAtomically(path, []byte("new content")); err != nil {
		t.Fatalf("writeFileAtomically: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new content" {
		t.Errorf("content = %q, want %q", got, "new content")
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(path); err != nil {
			t.Fatalf("Stat: %v", err)
		} else if st.Mode().Perm() != 0o600 {
			t.Errorf("mode = %o, want 600 preserved", st.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory contains %d entries, want 1: %v", len(entries), entries)
	}
}

func TestWriteFileAtomicallyDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	err := writeFileAtomically(dir, []byte("x"))
	if err == nil {
		t.Fatal("writeFileAtomically succeeded with a directory target, want error")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error = %q, want it to mention directory", err)
	}
}

func TestWriteFileAtomicallyMissingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "feed.rss")
	if err := writeFileAtomically(path, []byte("x")); err == nil {
		t.Fatal("writeFileAtomically succeeded with a missing parent directory, want error")
	}
}
