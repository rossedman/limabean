package cli

import (
	"strings"
	"testing"
)

func TestCp(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Stopped"
	h.world.vms["agent"] = "Running"
	out := h.must("", "cp", "-r", "./dist", "dev:code/")
	if !h.world.called("limactl start --tty=false --timeout=30m0s dev") || !h.world.called("limactl copy -r -- ./dist dev:code/") {
		t.Fatalf("calls: %v", h.world.calls)
	}
	if !strings.Contains(out, "Copied to dev:code/") {
		t.Errorf("out:\n%s", out)
	}
	h.must("", "cp", "dev:a.txt", "agent:b.txt")
	if !h.world.called("limactl copy -- dev:a.txt agent:b.txt") {
		t.Errorf("calls: %v", h.world.calls)
	}
	if _, err := h.run("", "cp", "./a:b", "/tmp/x"); err == nil || !strings.Contains(err.Error(), "name a VM path") {
		t.Errorf("no VM side: %v", err)
	}
	if _, err := h.run("", "cp", "x", "nope:"); err == nil || !strings.Contains(err.Error(), "no VM named") {
		t.Errorf("unknown VM: %v", err)
	}
	h.world.failStart = true
	h.world.vms["dev"] = "Stopped"
	if _, err := h.run("", "cp", "x", "dev:"); err == nil {
		t.Error("a VM that won't start should fail")
	}
}
