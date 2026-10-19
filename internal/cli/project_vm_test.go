package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Inside a project, commands that connect to or read from a VM default to
// the project's; ones that delete or roll back still need a name.
func TestProjectVMIsTheDefault(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "proj", "")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}
	h.world.vms["other"] = "Running"

	// No VM yet: say so, rather than picking another.
	if _, err := h.run("", "ssh"); err == nil || !strings.Contains(err.Error(), "proj has no VM yet: limabean up") {
		t.Fatalf("no VM: %v", err)
	}

	h.must("", "up", "--yes")
	h.must("", "ssh")
	if !h.world.called("exec ssh proj") {
		t.Errorf("ssh: %v", h.world.calls)
	}
	if out := h.must("", "ports"); !strings.Contains(out, "proj") {
		t.Errorf("ports:\n%s", out)
	}
	h.must("", "snapshot", "save")
	if !h.world.called("limactl stop proj") {
		t.Errorf("snapshot save: %v", h.world.calls)
	}

	// A name still wins, and rm and rebuild never default.
	h.must("", "ssh", "other")
	if !h.world.called("exec ssh other") {
		t.Errorf("named: %v", h.world.calls)
	}
	for _, cmd := range [][]string{{"rm"}, {"rebuild"}, {"snapshot", "restore"}} {
		if _, err := h.run("", cmd...); err == nil || !strings.Contains(err.Error(), "name a VM") {
			t.Errorf("%v without a name: %v", cmd, err)
		}
	}
	if h.world.vms["proj"] == "" {
		t.Fatal("proj was deleted")
	}
}
