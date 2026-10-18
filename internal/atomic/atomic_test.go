package atomic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(b) != "two\n" || info.Mode().Perm() != 0o600 {
		t.Errorf("got %q, %v", b, info.Mode())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary files left: %v", entries)
	}

	// A symlink stays a symlink; its target gets the new content.
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced")
	}
	if b, _ := os.ReadFile(path); string(b) != "three\n" {
		t.Errorf("target: %q", b)
	}

	if err := WriteFile(filepath.Join(dir, "missing", "x"), nil, 0o644); err == nil {
		t.Error("a missing directory should fail")
	}
}
