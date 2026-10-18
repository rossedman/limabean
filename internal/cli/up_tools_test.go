package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpTools(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "tooly", "tools:\n  terraform: 1.9.8\n  \"npm:@biomejs/biome\": 2.1.2\napt: [libpq-dev, jq, libpq-dev]\n")

	// On create: in the plan, and written next to the packs' lists.
	out := h.must("", "up", "--file", file, "--yes")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(out, "tools: npm:@biomejs/biome 2.1.2, terraform 1.9.8") || !strings.Contains(out, "apt: jq, libpq-dev") ||
		!strings.Contains(out, "Project tools: npm:@biomejs/biome 2.1.2, terraform 1.9.8") {
		t.Fatalf("plan:\n%s", out)
	}
	for _, want := range []string{
		`"path": "/etc/limabean/mise/90-project.toml", "content": "# From tools:`,
		`[tools]\n\"npm:@biomejs/biome\" = \"2.1.2\"\n\"terraform\" = \"1.9.8\"\n"`,
		`"path": "/etc/limabean/apt/_project.list", "content": "jq\nlibpq-dev\n"`,
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("limactl start is missing %s\n%s", want, calls)
		}
	}

	// The same file again: nothing to do.
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "tooly is up to date") {
		t.Errorf("again:\n%s", out)
	}

	// A change rebuilds, keeping ~/code, and asks first.
	rewrite(t, file, "tools:\n  terraform: 1.10.0\n")
	if out := h.must("", "up", "--file", file, "--preview"); !strings.Contains(out, "tools: npm:@biomejs/biome 2.1.2, terraform 1.9.8 → terraform 1.10.0 (rebuild, keeping ~/code)") ||
		!strings.Contains(out, "apt: jq, libpq-dev → none") || h.world.called("limactl delete") {
		t.Fatalf("preview:\n%s", out)
	}
	if _, err := h.run("", "up", "--file", file); err == nil {
		t.Error("a change needs --yes without a terminal")
	}
	h.world.calls = nil
	h.must("", "up", "--file", file, "--yes")
	calls = strings.Join(h.world.calls, "\n")
	if !h.world.called("limactl delete") || !strings.Contains(calls, `\"terraform\" = \"1.10.0\"`) || strings.Contains(calls, "_project.list") {
		t.Fatalf("rebuild:\n%s", calls)
	}

	// limabean rebuild keeps them.
	h.world.calls = nil
	h.must("", "rebuild", "tooly", "--force")
	if !strings.Contains(strings.Join(h.world.calls, "\n"), `\"terraform\" = \"1.10.0\"`) {
		t.Errorf("rebuild dropped the tools: %v", h.world.calls)
	}

	// None left: forgotten, and rm tidies up.
	rewrite(t, file, "")
	h.must("", "up", "--file", file, "--yes")
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "tooly.tools")); err == nil {
		t.Error("the tools should be forgotten")
	}
	rewrite(t, file, "apt: [jq]\n")
	h.must("", "up", "--file", file, "--yes")

	// A clone has the same tools, so its rebuilds keep them.
	h.must("", "clone", "tooly", "tooly-2")
	if b, _ := os.ReadFile(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "tooly-2.tools")); string(b) != `{"apt":["jq"]}` {
		t.Errorf("clone's tools: %q", b)
	}
	h.must("", "rm", "tooly", "--force")
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "tooly.tools")); err == nil {
		t.Error("rm should remove the tools' state")
	}
}

// A VM copied from an image gets the project's tools too: they're the VM's
// own settings, not the image's.
func TestUpToolsFromAnImage(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_IMAGES", "yes")
	h.must("", "new", "first")
	h.must("", "images", "build", "--if-needed", "--env", "scratch")
	file := projectFile(t, "copy", "tools:\n  just: 1.40.0\n")
	h.world.calls = nil
	h.must("", "up", "--file", file, "--yes")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(calls, "limactl clone") || !strings.Contains(calls, `\"just\" = \"1.40.0\"`) {
		t.Fatalf("clone: %s", calls)
	}
}
