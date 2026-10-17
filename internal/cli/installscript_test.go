package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/secrets"
)

// The install script runs for real with sh, in a fake home.
func runInstallScript(t *testing.T, home, stdin string, keep []string) string {
	t.Helper()
	cmd := exec.Command("sh", "-c", installScript(keep))
	cmd.Env = append(os.Environ(), "HOME="+home, "PATH=/usr/bin:/bin")
	cmd.Stdin = strings.NewReader(stdin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	b, err := os.ReadFile(filepath.Join(home, ".config", "limabean", "secrets.env"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallScriptKeepsUnresolvedValues(t *testing.T) {
	home := t.TempDir()
	tricky := `it's "x" $y \z`
	first := secrets.Render([]secrets.Value{{Name: "TOKEN", Value: tricky}, {Name: "GONE", Value: "old"}, {Name: "A", Value: "1"}})
	runInstallScript(t, home, first, nil)

	// TOKEN couldn't be resolved this time; GONE was removed from the refs;
	// A has a new value.
	got := runInstallScript(t, home, secrets.Render([]secrets.Value{{Name: "A", Value: "2"}}), []string{"TOKEN", "NEVER_SET"})
	if strings.Contains(got, "GONE") || strings.Contains(got, "NEVER_SET") {
		t.Errorf("removed or never-set names carried over:\n%s", got)
	}
	out, err := exec.Command("sh", "-c", got+`printf '%s|%s' "$TOKEN" "$A"`).Output()
	if err != nil || string(out) != tricky+"|2" {
		t.Fatalf("sourced = %q, %v\nfile:\n%s", out, err, got)
	}
	fi, _ := os.Stat(filepath.Join(home, ".config", "limabean", "secrets.env"))
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
}

func TestInstallScriptWithNoOldFile(t *testing.T) {
	got := runInstallScript(t, t.TempDir(), secrets.Render([]secrets.Value{{Name: "A", Value: "1"}}), []string{"MISSING"})
	if !strings.Contains(got, "export A='1'") || strings.Contains(got, "MISSING") {
		t.Fatalf("got:\n%s", got)
	}
}
