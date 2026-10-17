package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPathAndDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if DefaultPath() != "/xdg/limabean/config" {
		t.Fatalf("XDG DefaultPath = %s", DefaultPath())
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if !strings.HasSuffix(DefaultPath(), filepath.Join(".config", "limabean", "config")) {
		t.Fatalf("DefaultPath = %s", DefaultPath())
	}
	c, err := Load(New(), "")
	if err != nil || c.Path != DefaultPath() || c.Dir() != filepath.Dir(DefaultPath()) {
		t.Fatalf("Load(\"\") = %+v, %v", c, err)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(New(), dir); err == nil { // a directory isn't a config file
		t.Error("loading a directory should fail")
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	if _, err := Load(New(), filepath.Join(locked, "config")); err == nil {
		t.Error("an unreadable directory should fail")
	}
	t.Setenv("LIMABEAN_CPUS", "-2")
	if _, err := Load(New(), filepath.Join(dir, "none")); err == nil {
		t.Error("negative CPUS should fail")
	}
}

func TestSaveErrorsAndYes(t *testing.T) {
	dir := t.TempDir()
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	if err := (&Config{Path: filepath.Join(ro, "sub", "config")}).Save(); err == nil {
		t.Error("Save under a read-only directory should fail")
	}
	if err := (&Config{Path: filepath.Join(ro, "config")}).Save(); err == nil {
		t.Error("Save into a read-only directory should fail")
	}
	for _, v := range []string{"yes", "Y", "true", "1", "on"} {
		if !yes(v) {
			t.Errorf("yes(%q) = false", v)
		}
	}
	if yes("no") || yes("") {
		t.Error("no and empty aren't yes")
	}
}
