package templates

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestRootErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Dirs{Cache: filepath.Join(file, "cache")}).Root(embedded()); err == nil {
		t.Error("a cache path under a file should fail")
	}
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	if _, err := (Dirs{Cache: ro}).Root(embedded()); err == nil {
		t.Error("a read-only cache should fail")
	}
}

// A cache directory left behind by an interrupted run (no .complete marker)
// makes the final rename fail; Root reports it rather than using a partial copy.
func TestRootInterruptedCache(t *testing.T) {
	cache := t.TempDir()
	root, err := Dirs{Cache: cache}.Root(embedded())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".complete")); err != nil {
		t.Fatal(err)
	}
	if _, err := (Dirs{Cache: cache}).Root(embedded()); err == nil {
		t.Fatal("a half-written cache dir should make Root fail, not be reused")
	}
}

func TestUserDirErrors(t *testing.T) {
	user := t.TempDir()
	if err := os.Chmod(user, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(user, 0o700) })
	if _, err := (Dirs{Cache: t.TempDir(), User: user}).Root(embedded()); err == nil {
		t.Error("an unreadable user templates dir should fail")
	}
	_ = os.Chmod(user, 0o700)

	// The copy of a user env can't be written: a directory is in the way.
	if err := os.WriteFile(filepath.Join(user, "mine.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	root, err := Dirs{Cache: cache}.Root(embedded())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "user-mine.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Dirs{Cache: cache, User: user}).Root(embedded()); err == nil {
		t.Error("an unwritable user env copy should fail")
	}
}

func TestCopyUserEdgeCases(t *testing.T) {
	user := t.TempDir()
	for name, body := range map[string]string{"layer-x.yaml": "skip", "notes.txt": "skip", "mine.yaml": "keep"} {
		if err := os.WriteFile(filepath.Join(user, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(user, "dir.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := Dirs{Cache: t.TempDir(), User: user}.Root(embedded())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "user-mine.yaml")); err != nil {
		t.Error("user env not copied")
	}
	for _, skipped := range []string{"user-layer-x.yaml", "user-notes.txt", "user-dir.yaml"} {
		if _, err := os.Stat(filepath.Join(root, skipped)); err == nil {
			t.Errorf("%s shouldn't be copied", skipped)
		}
	}
	// Unreadable user env.
	if err := os.Chmod(filepath.Join(user, "mine.yaml"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(user, "mine.yaml"), 0o644) })
	if _, err := (Dirs{Cache: t.TempDir(), User: user}).Root(embedded()); err == nil {
		t.Error("an unreadable user env should fail")
	}
	// User dir that's a file.
	if _, err := (Dirs{Cache: t.TempDir(), User: filepath.Join(user, "mine.yaml")}).Root(embedded()); err == nil {
		t.Error("a user dir that's a file should fail")
	}
	if _, err := Envs(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("Envs on a missing dir should fail")
	}
}

// failingFS refuses to open one file, so hashing the embedded tree fails.
type failingFS struct {
	fs.FS
	bad string
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.bad {
		return nil, fs.ErrPermission
	}
	return f.FS.Open(name)
}

func TestUnreadableEmbeddedFile(t *testing.T) {
	_, err := Dirs{Cache: t.TempDir()}.Root(failingFS{FS: embedded(), bad: "templates/scratch.yaml"})
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want a permission error", err)
	}
}
