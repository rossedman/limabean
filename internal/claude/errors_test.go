package claude

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	if !strings.HasSuffix(DefaultPath(), filepath.Join(".claude", "settings.json")) {
		t.Fatalf("DefaultPath = %s", DefaultPath())
	}
}

func TestNotAnObjectIsInvalid(t *testing.T) {
	for _, bad := range []string{`[]`, `"x"`, `{"a":1} {"b":2}`, `{1: 2}`, `{"a": }`, `{"a": 1`} {
		s := Settings{Path: filepath.Join(t.TempDir(), "settings.json")}
		writeFile(t, s.Path, bad)
		if _, err := s.VMs(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: VMs err = %v, want ErrInvalid", bad, err)
		}
		if err := s.Remove("dev"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: Remove err = %v, want ErrInvalid", bad, err)
		}
		if got := readFile(t, s.Path); got != bad {
			t.Errorf("%q: file changed to %q", bad, got)
		}
	}
}

func TestSSHConfigsNotAList(t *testing.T) {
	s := Settings{Path: filepath.Join(t.TempDir(), "settings.json")}
	writeFile(t, s.Path, `{"sshConfigs": {"oops": true}}`)
	if _, err := s.VMs(); err == nil || !strings.Contains(err.Error(), "isn't a list") {
		t.Fatalf("VMs err = %v", err)
	}
	if err := s.Add("dev"); err == nil {
		t.Fatal("Add should refuse")
	}
}

func TestNullAndDuplicateKeys(t *testing.T) {
	s := Settings{Path: filepath.Join(t.TempDir(), "settings.json")}
	writeFile(t, s.Path, `{"sshConfigs": null, "a": 1, "a": 2}`)
	if vms, err := s.VMs(); err != nil || len(vms) != 0 {
		t.Fatalf("VMs = %v, %v", vms, err)
	}
	if err := s.Add("dev"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, s.Path); strings.Count(got, `"a"`) != 1 || !strings.Contains(got, `"a": 2`) {
		t.Fatalf("duplicate key handling:\n%s", got)
	}
}

func TestUnreadableAndUnwritable(t *testing.T) {
	dir := t.TempDir()
	s := Settings{Path: dir} // a directory: reading fails
	if _, err := s.VMs(); err == nil {
		t.Error("reading a directory should fail")
	}
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	if err := (Settings{Path: filepath.Join(ro, "sub", "settings.json")}).Add("dev"); err == nil {
		t.Error("creating a directory under a read-only one should fail")
	}
	if err := (Settings{Path: filepath.Join(ro, "settings.json")}).Add("dev"); err == nil {
		t.Error("writing into a read-only directory should fail")
	}
}
