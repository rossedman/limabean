package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClone(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "dev", "--secret", "OWN=x", "--with", "rust", "--allow", "github", "--locked")
	h.world.writeFile(filepath.Join("_disks", codeDiskName("dev"), "datadisk"), "repos of dev")
	h.must("", "snapshot", "save", "dev", "before")
	h.world.calls = nil

	out := h.must("", "clone", "dev", "dev-2")
	calls := strings.Join(h.world.calls, "\n")
	for _, want := range []string{
		"limactl stop dev",
		"limactl disk create " + codeDiskName("dev-2"),
		"clone datadisk",
		`.param.networkDir = "` + filepath.Join(h.dir, "network", "dev-2"),
		`select(.name == "` + codeDiskName("dev") + `") | .name) = "` + codeDiskName("dev-2") + `"`,
		"dev dev-2",
		"limactl start --tty=false dev-2",
		"limactl start --tty=false dev\n",
		"lima-dev-2 sudo -n systemctl start limabean-network.service",
	} {
		if !strings.Contains(calls+"\n", want) {
			t.Errorf("calls missing %q:\n%s", want, calls)
		}
	}
	if strings.Index(calls, "limactl stop dev") > strings.Index(calls, "limactl clone") {
		t.Errorf("the source must be stopped before cloning:\n%s", calls)
	}
	if got := h.world.readFile(filepath.Join("_disks", codeDiskName("dev-2"), "datadisk")); got != "repos of dev" {
		t.Errorf("code disk = %q", got)
	}
	if h.world.vms["dev"] != "Running" || h.world.vms["dev-2"] != "Running" {
		t.Errorf("vms = %v", h.world.vms)
	}
	if !strings.Contains(h.read("secrets.dev-2.env"), "OWN=x") || !strings.Contains(h.read(filepath.Join("network", "dev-2", "network.conf")), "locked") {
		t.Errorf("the copy should have dev's secrets and policy")
	}
	if b, _ := os.ReadFile(filepath.Join(filepath.Dir(h.dir), "state", "limabean", "dev-2.packs")); !strings.Contains(string(b), "rust") {
		t.Errorf("packs not copied: %q", b)
	}
	if !strings.Contains(out, "Copied dev to dev-2") || !strings.Contains(out, "dev-2 is ready") {
		t.Errorf("out:\n%s", out)
	}

	for _, args := range [][]string{{"clone", "dev", "dev-2"}, {"clone", "dev", "bad_name"}, {"clone", "nope", "x"}} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("%v should fail", args)
		}
	}
	h.world.disks[codeDiskName("stale")] = true
	if _, err := h.run("", "clone", "dev", "stale"); err == nil || !strings.Contains(err.Error(), "left over") {
		t.Errorf("a leftover disk: %v", err)
	}

	h.world.failLimaClone = true
	if _, err := h.run("", "clone", "dev", "dev-3"); err == nil {
		t.Fatal("a failed limactl clone should fail")
	}
	if h.world.disks[codeDiskName("dev-3")] || fileExists(filepath.Join(h.dir, "network", "dev-3")) {
		t.Errorf("a failed clone should leave nothing behind")
	}
	if h.world.vms["dev"] != "Running" {
		t.Errorf("dev should be started again after a failed clone")
	}
	h.world.failLimaClone = false
	h.world.failClone = true
	if _, err := h.run("", "clone", "dev", "dev-4"); err == nil || h.world.disks[codeDiskName("dev-4")] {
		t.Errorf("a failed disk copy should fail and remove the new disk: %v", err)
	}
}

func TestCloneWithoutCodeDisk(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.world.vms["old"] = "Stopped"
	out := h.must("", "clone", "old", "new")
	if h.world.called("limactl disk create") || strings.Contains(strings.Join(h.world.calls, "\n"), "codeDisk") {
		t.Errorf("no code disk to copy: %v", h.world.calls)
	}
	if h.world.called("limactl start --tty=false old") || !strings.Contains(out, "new is ready") {
		t.Errorf("a stopped source stays stopped:\n%s", out)
	}
}
