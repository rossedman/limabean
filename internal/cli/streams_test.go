package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/ui"
)

// stdout carries only data; everything else goes to stderr.
func TestStreamsAreTyped(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	split := func(args ...string) (string, string) {
		t.Helper()
		var out, errs bytes.Buffer
		if _, err := h.runUI(ui.NewPlainStreams(&out, &errs), &bytes.Buffer{}, "", args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, errs.String())
		}
		return out.String(), errs.String()
	}

	out, errs := split("up", "dev", "--with", "go")
	if out != "" || !strings.Contains(errs, "dev is ready") {
		t.Fatalf("up: stdout %q, stderr %q", out, errs)
	}
	out, _ = split("ls", "--json")
	var list []lsEntry
	if err := json.Unmarshal([]byte(out), &list); err != nil || len(list) != 1 {
		t.Fatalf("ls --json stdout isn't only JSON: %q", out)
	}
	if out, _ := split("ls"); !strings.HasPrefix(out, "NAME") {
		t.Errorf("ls: %q", out)
	}

	// A report's lines are its result, so they're on stdout.
	file := projectFile(t, "rep", "")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}
	if out, _ := split("ps"); !strings.Contains(out, "not created yet") {
		t.Errorf("ps: stdout %q", out)
	}

	// Empty results: nothing on stdout, a note on stderr.
	chdirTemp(t)
	h.world.vms = map[string]string{}
	if out, errs := split("ls"); out != "" || !strings.Contains(errs, "No VMs yet") {
		t.Errorf("empty ls: stdout %q stderr %q", out, errs)
	}
	if out, _ := split("ls", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("empty ls --json: %q", out)
	}
}

func TestQuietAndVerboseFlags(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	if out := h.must("", "up", "dev", "-q"); strings.Contains(out, "dev is ready") {
		t.Errorf("--quiet:\n%s", out)
	}
	if out := h.must("", "down", "dev", "-q", "-v", "--no-color"); !strings.Contains(out, "Stopped dev") || !strings.Contains(out, "$ limactl stop dev") {
		t.Errorf("--verbose wins over --quiet:\n%s", out)
	}
}
