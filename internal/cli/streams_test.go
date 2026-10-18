package cli

import (
	"bytes"
	"encoding/json"
	"errors"
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

// --yes skips the confirmation only; the unsaved-work check still runs.
// --force skips both. Old flag names keep working, hidden.
func TestYesSkipsOnlyTheQuestion(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.must("", "new", "a")
	h.world.unsaved = "~/code/x: stashes\n"
	if _, err := h.runTTY(nil, "rm", "a", "--yes"); !errors.Is(err, errUnsaved) {
		t.Fatalf("rm --yes with unsaved work: %v", err)
	}
	if _, err := h.runTTY(nil, "rebuild", "a", "-y"); !errors.Is(err, errUnsaved) {
		t.Fatalf("rebuild -y with unsaved work: %v", err)
	}
	h.world.unsaved = ""
	if out, err := h.runTTY(nil, "rebuild", "a", "-y"); err != nil || !strings.Contains(out, "a was rebuilt") {
		t.Fatalf("rebuild -y: %v\n%s", err, out)
	}
	if out, err := h.runTTY(nil, "rm", "a", "--yes"); err != nil || !strings.Contains(out, "Deleted a") {
		t.Fatalf("rm --yes: %v\n%s", err, out)
	}

	for _, c := range []struct{ cmd, hidden string }{
		{"up", "dry-run"}, {"down", "dry-run"}, {"bump", "dry-run"}, {"recipes", "force"}, {"snapshot restore", "force"},
	} {
		out := h.must("", append(strings.Fields(c.cmd), "--help")...)
		if strings.Contains(out, "--"+c.hidden) {
			t.Errorf("%s --help shows --%s", c.cmd, c.hidden)
		}
	}
}

func TestConfigShowsSources(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CPUS", "4")
	var got []settingEntry
	if err := json.Unmarshal([]byte(h.must("", "config", "--json", "--templates", t.TempDir())), &got); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, e := range got {
		sources[e.Name] = e.Value + " " + e.Source
	}
	if sources["CPUS"] != "4 env" || sources["CODE_DIR"] != "~/code default" || !strings.HasSuffix(sources["TEMPLATES"], " flag") {
		t.Errorf("sources: %v", sources)
	}
	h.must("", "init")
	if out := h.must("", "config"); !strings.Contains(out, "CODE_DIR") || !strings.Contains(out, `"~/code"`) || !strings.Contains(out, "file") {
		t.Errorf("table:\n%s", out)
	}
}

// Hidden commands kept for scripts say what replaced them, except for
// autostop agents' stop --idle and with --quiet.
func TestOldCommandNotes(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Stopped"
	if out := h.must("", "start", "dev"); !strings.Contains(out, `"limabean start" is now "limabean up <name>"`) {
		t.Errorf("start:\n%s", out)
	}
	if out := h.must("", "stop", "dev"); !strings.Contains(out, `"limabean stop" is now "limabean down <name>"`) {
		t.Errorf("stop:\n%s", out)
	}
	if out := h.must("", "stop", "--idle", "2h"); strings.Contains(out, "is now") {
		t.Errorf("stop --idle:\n%s", out)
	}
	if out := h.must("", "stop", "dev", "-q"); strings.Contains(out, "is now") {
		t.Errorf("stop -q:\n%s", out)
	}
}
