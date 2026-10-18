package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPs(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "shop", "ports: [3000, 9999]\nstart: npm run dev\n")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}

	// Not created yet.
	out := h.must("", "ps")
	if !strings.Contains(out, "not created yet: limabean up") || !strings.Contains(out, "port 3000") || !strings.Contains(out, "VM not running") {
		t.Fatalf("no VM:\n%s", out)
	}

	h.must("", "up", "--yes")
	h.world.ss = ssOutput
	h.world.macPorts[3000] = true
	h.world.startRunning = true
	out = h.must("", "ps")
	for _, want := range []string{"Running", "running (limabean logs --start -f)", "port 3000 vite", "http://localhost:3000", "port 9999", "nothing listening", "port 5432", "not forwarded", "up to date"} {
		if !strings.Contains(out, want) {
			t.Errorf("running: missing %q in\n%s", want, out)
		}
	}

	// Pending changes, a refusal, and start changed since approval.
	rewrite(t, file, "ports: [3000]\ncpus: 6\nstart: npm run serve\n")
	h.world.startRunning = false
	out = h.must("", "ps")
	if !strings.Contains(out, "changes for limabean up:") || !strings.Contains(out, "cpus: 4 → 6") || !strings.Contains(out, "changed since you approved it") {
		t.Fatalf("changes:\n%s", out)
	}
	if h.world.called("limactl edit") {
		t.Error("ps changed something")
	}

	// Stopped, and as JSON.
	rewrite(t, file, "ports: [3000]\nstart: npm run dev\nnetwork:\n  allow: [github]\n")
	h.must("", "up", "--yes")
	rewrite(t, file, "ports: [3000]\nstart: npm run dev\nnetwork:\n  allow: [github, npm]\n")
	h.world.vms["shop"] = "Stopped"
	var e psEntry
	if err := json.Unmarshal([]byte(h.must("", "ps", "--json")), &e); err != nil {
		t.Fatal(err)
	}
	if e.Status != "Stopped" || e.Start != "stopped" || len(e.Ports) != 1 || e.Ports[0].Listening || len(e.Widening) != 1 || !strings.Contains(e.Widening[0], "network allow shop") {
		t.Fatalf("json: %+v", e)
	}
	if out := h.must("", "ps"); !strings.Contains(out, "not running; limabean up starts it") || !strings.Contains(out, "only you can change") {
		t.Errorf("stopped:\n%s", out)
	}

	// No start in the file; outside a project.
	rewrite(t, file, "network:\n  allow: [github]\n")
	h.world.vms["shop"] = "Running"
	if out := h.must("", "ps"); !strings.Contains(out, "none in the file") {
		t.Errorf("no start:\n%s", out)
	}
	chdirTemp(t)
	if _, err := h.run("", "ps"); err == nil || !strings.Contains(err.Error(), "limabean ls") {
		t.Errorf("outside a project: %v", err)
	}
}
