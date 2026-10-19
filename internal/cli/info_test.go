package cli

import (
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"

	// A VM made before limabean-info.
	if _, err := h.run("", "info", "dev"); err == nil || exitOf(err) != exitNotFound || !strings.Contains(err.Error(), "rebuild") {
		t.Errorf("old VM: %v", err)
	}

	h.world.info = "dev (scratch), a limabean VM\n  packs:     core\n"
	if out := h.must("", "info", "dev"); !strings.Contains(out, "dev (scratch), a limabean VM") || !h.world.called("ssh") {
		t.Errorf("info:\n%s", out)
	}
	if !strings.Contains(strings.Join(h.world.calls, "\n"), "then limabean-info; else") {
		t.Errorf("plain info ran: %v", h.world.calls)
	}

	// --json passes on only JSON.
	h.world.info = `{"name":"dev","network":{"locked":true}}`
	if out := h.must("", "info", "dev", "--json"); !strings.Contains(out, "\"locked\": true") {
		t.Errorf("--json:\n%s", out)
	}
	h.world.info = "\x1b]0;not json"
	if _, err := h.run("", "info", "dev", "--json"); err == nil || !strings.Contains(err.Error(), "didn't print JSON") {
		t.Errorf("not JSON: %v", err)
	}

	// Stopped: says so rather than starting it.
	h.world.vms["dev"] = "Stopped"
	if _, err := h.run("", "info", "dev"); err == nil || !strings.Contains(err.Error(), "is stopped") || h.world.called("limactl start") {
		t.Errorf("stopped: %v", err)
	}

	// In a project's checkout, its VM.
	h.world.vms["web"] = "Running"
	h.world.info = `{"name":"web"}`
	chdirTemp(t)
	rewrite(t, ".limabean.yaml", "name: web\n")
	if out := h.must("", "info", "--json"); !strings.Contains(out, `"name": "web"`) {
		t.Errorf("project VM:\n%s", out)
	}
}

func TestMCPVMInfo(t *testing.T) {
	h := newHarness(t)
	h.world.vms["agent"] = "Running"
	h.world.info = `{"name":"agent","secrets":["GITHUB_TOKEN"]}`
	cs, _ := h.mcpClient(false)
	if out, isErr := call(t, cs, "vm_info", map[string]any{"name": "agent"}); isErr || !strings.Contains(out, `"GITHUB_TOKEN"`) {
		t.Errorf("vm_info: %v\n%s", isErr, out)
	}
	if out, isErr := call(t, cs, "vm_info", map[string]any{"name": "--force"}); !isErr {
		t.Errorf("a name that's a flag:\n%s", out)
	}
}
