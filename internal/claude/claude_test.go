package claude

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const existing = `{
  "zeta": 1,
  "enabledPlugins": {"a@b": true},
  "sshConfigs": [
    {"id": "mine", "name": "Hand-made", "sshHost": "example.com", "sshPort": 2222}
  ],
  "alpha": "keep"
}
`

func TestAddRemoveKeepsOtherSettings(t *testing.T) {
	s := Settings{Path: filepath.Join(t.TempDir(), "settings.json")}
	writeFile(t, s.Path, existing)

	if err := s.Add("dev"); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("dev"); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := s.Add("box"); err != nil {
		t.Fatal(err)
	}
	vms, err := s.VMs()
	if err != nil || strings.Join(vms, ",") != "dev,box" {
		t.Fatalf("VMs = %v, %v", vms, err)
	}

	out := readFile(t, s.Path)
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if doc["alpha"] != "keep" || doc["zeta"] != float64(1) {
		t.Errorf("other keys changed:\n%s", out)
	}
	if strings.Index(out, `"zeta"`) > strings.Index(out, `"alpha"`) {
		t.Errorf("top-level key order changed:\n%s", out)
	}
	if !strings.Contains(out, `"sshPort": 2222`) {
		t.Errorf("hand-made entry changed:\n%s", out)
	}
	if !strings.Contains(out, `"sshHost": "lima-dev"`) || !strings.Contains(out, `"name": "dev (limabean)"`) {
		t.Errorf("dev entry missing:\n%s", out)
	}

	if err := s.Remove("dev"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("never-existed"); err != nil {
		t.Fatal(err)
	}
	vms, _ = s.VMs()
	if strings.Join(vms, ",") != "box" {
		t.Errorf("after remove VMs = %v", vms)
	}
	if !strings.Contains(readFile(t, s.Path), `"id": "mine"`) {
		t.Errorf("hand-made entry removed")
	}
}

func TestMissingOrEmptyFileIsCreated(t *testing.T) {
	for _, initial := range []string{"", "  \n"} {
		s := Settings{Path: filepath.Join(t.TempDir(), "sub", "settings.json")}
		if initial != "" {
			writeFile(t, s.Path, initial)
		}
		if err := s.Add("dev"); err != nil {
			t.Fatal(err)
		}
		var doc struct{ SSHConfigs []Entry }
		if err := json.Unmarshal([]byte(readFile(t, s.Path)), &doc); err != nil || len(doc.SSHConfigs) != 1 {
			t.Fatalf("got %s (%v)", readFile(t, s.Path), err)
		}
	}
}

func TestInvalidJSONIsLeftAlone(t *testing.T) {
	s := Settings{Path: filepath.Join(t.TempDir(), "settings.json")}
	writeFile(t, s.Path, "{bad")
	if err := s.Add("dev"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if got := readFile(t, s.Path); got != "{bad" {
		t.Errorf("file changed to %q", got)
	}
}

func TestSymlinkIsKept(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles-settings.json")
	writeFile(t, target, `{"a": 1}`)
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := (Settings{Path: link}).Add("dev"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v", err)
	}
	if !strings.Contains(readFile(t, target), "lima-dev") {
		t.Errorf("target not updated: %s", readFile(t, target))
	}
}

func TestPermissionsAreKept(t *testing.T) {
	s := Settings{Path: filepath.Join(t.TempDir(), "settings.json")}
	writeFile(t, s.Path, `{}`)
	if err := os.Chmod(s.Path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("dev"); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(s.Path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
}

func writeFile(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
