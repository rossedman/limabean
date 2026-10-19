package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpdateNotice(t *testing.T) {
	h := newHarness(t)
	h.version = "v0.3.0"
	state := filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "update.json")

	// The first command in a terminal starts a check in the background, once.
	if _, err := h.runTTY(nil, "ls"); err != nil {
		t.Fatal(err)
	}
	if !h.world.called("spawn update-check") {
		t.Fatalf("no check: %v", h.world.calls)
	}
	h.world.calls = nil
	if _, err := h.runTTY(nil, "ls"); err != nil || h.world.called("spawn update-check") {
		t.Errorf("checked again within a day: %v %v", err, h.world.calls)
	}

	// The background check records the latest release.
	h.world.latestRelease = "v0.4.1"
	h.must("", "update-check")
	var s updateState
	if b, err := os.ReadFile(state); err != nil || json.Unmarshal(b, &s) != nil || s.Latest != "v0.4.1" {
		t.Fatalf("state: %s, %v", b, err)
	}

	// Then commands in a terminal say so; scripts, --json, -q and MCP don't.
	if out, _ := h.runTTY(nil, "ls"); !strings.Contains(out, "limabean 0.4.1 is out (this is 0.3.0): brew upgrade limabean") {
		t.Errorf("terminal:\n%s", out)
	}
	for _, args := range [][]string{{"ls"}, {"ls", "--json"}, {"ls", "-q"}} {
		run := h.runTTY
		if len(args) == 1 {
			run = func(_ []string, args ...string) (string, error) { return h.run("", args...) }
		}
		if out, _ := run(nil, args...); strings.Contains(out, "is out") {
			t.Errorf("%v:\n%s", args, out)
		}
	}
	t.Setenv("LIMABEAN_UPDATE_CHECK", "no")
	if out, _ := h.runTTY(nil, "ls"); strings.Contains(out, "is out") {
		t.Errorf("UPDATE_CHECK=no:\n%s", out)
	}
	t.Setenv("LIMABEAN_UPDATE_CHECK", "")
	os.Unsetenv("LIMABEAN_UPDATE_CHECK")

	// Up to date, a dev build, or a reply that isn't a version: nothing.
	h.world.latestRelease = "v0.3.0"
	h.must("", "update-check")
	if out, _ := h.runTTY(nil, "ls"); strings.Contains(out, "is out") {
		t.Errorf("up to date:\n%s", out)
	}
	h.world.latestRelease = "v9.9.9\u001b[2J"
	if _, err := h.run("", "update-check"); err == nil {
		t.Error("a tag that isn't a version should be refused")
	}
	h.world.latestRelease = ""
	if _, err := h.run("", "update-check"); err == nil {
		t.Error("no release: no error")
	}
	_ = os.WriteFile(state, []byte(`{"checked":"`+time.Now().Format(time.RFC3339)+`","latest":"v9.0.0"}`), 0o644)
	h.version = "dev"
	if out, _ := h.runTTY(nil, "ls"); strings.Contains(out, "is out") {
		t.Errorf("dev build:\n%s", out)
	}
}
