package cli

import (
	"strings"
	"testing"
)

func TestUpVars(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "envy", "vars:\n  NODE_ENV: development\n  GREETING: \"it's here\"\n")

	// On create: in the plan, and written to the VM before setup.
	out := h.must("", "up", "--file", file, "--yes")
	written := h.world.sshStdin["envy.env"]
	if !strings.Contains(out, "  NODE_ENV=development") || !strings.Contains(written, "export GREETING='it'\\''s here'\n") || !strings.Contains(written, "export NODE_ENV='development'\n") {
		t.Fatalf("create:\n%s\n%q", out, written)
	}

	// Unchanged: nothing to apply.
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "up to date") {
		t.Errorf("unchanged:\n%s", out)
	}

	// Changed: listed, asked about, applied to the running VM without a rebuild.
	rewrite(t, file, "vars:\n  NODE_ENV: production\n  PORT: \"3000\"\n")
	if _, err := h.run("", "up", "--file", file); err == nil {
		t.Fatal("changed vars need --yes without a terminal")
	}
	h.world.calls = nil
	out = h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(out, "vars: set NODE_ENV=production") || !strings.Contains(out, "vars: set PORT=3000") ||
		!strings.Contains(out, "vars: unset GREETING") || h.world.called("limactl delete") || !strings.Contains(h.world.sshStdin["envy.env"], "export PORT='3000'") {
		t.Fatalf("changed:\n%s\n%q", out, h.world.sshStdin["envy.env"])
	}

	// A rebuild for other reasons writes it again, on the fresh system disk.
	rewrite(t, file, "packs: [go]\nvars:\n  NODE_ENV: production\n  PORT: \"3000\"\n")
	delete(h.world.sshStdin, "envy.env")
	h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(h.world.sshStdin["envy.env"], "NODE_ENV") {
		t.Error("the vars weren't written again after the rebuild")
	}

	// A VM whose env.sh predates vars: says so.
	h.world.oldEnvSh = true
	rewrite(t, file, "packs: [go]\nvars:\n  NODE_ENV: test\n")
	if out := h.must("", "up", "--file", file, "--yes"); !strings.Contains(out, "made before vars: worked") {
		t.Errorf("old env.sh:\n%s", out)
	}
	h.world.oldEnvSh = false

	// None any more: the file goes.
	rewrite(t, file, "packs: [go]\n")
	if out := h.must("", "up", "--file", file, "--yes"); !strings.Contains(out, "vars: unset NODE_ENV") || h.world.sshStdin["envy.env"] != "" {
		t.Errorf("removed:\n%s\n%q", out, h.world.sshStdin["envy.env"])
	}
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "up to date") {
		t.Errorf("after removing:\n%s", out)
	}
}
