package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUp(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	dir := filepath.Join(t.TempDir(), "myapp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("sh", "-c", "cd '"+dir+"' && git init -q && git remote add origin git@github.com:me/myapp.git").CombinedOutput(); err != nil {
		t.Fatalf("git: %v %s", err, out)
	}
	file := filepath.Join(dir, ".limabean.yaml")
	if err := os.WriteFile(file, []byte(`packs: [rust]
cpus: 6
memory: 12
network:
  allow: [github]
  locked: true
secrets: [NPM_TOKEN, SHARED, MISSING]
ports: [3000]
setup: |
  make deps
  make db
`), 0o644); err != nil {
		t.Fatal(err)
	}
	h.must("", "secrets", "set", "SHARED", "plain", "--no-sync")
	h.must("", "secrets", "set", "NPM_TOKEN", "op://Work/npm/token", "--vm", "other", "--no-sync")
	h.world.ghq = "/home/me.linux/code/github.com/me/myapp\n"

	// A file from a repository isn't acted on without a yes.
	out, err := h.run("", "up", "--file", file)
	if err == nil || !strings.Contains(err.Error(), "--yes") || len(h.world.vms) != 0 {
		t.Fatalf("up without --yes: %v\n%s", err, out)
	}
	for _, want := range []string{"myapp", "clone: github.com/me/myapp", "make deps", ", locked", "Not set up on this Mac, so the VM won't get them: MISSING"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan missing %q:\n%s", want, out)
		}
	}
	if out, err := h.runTTY([]string{"n"}, "up", "--file", file); err != nil || !strings.Contains(out, "Nothing created") || len(h.world.vms) != 0 {
		t.Fatalf("declined: %v\n%s", err, out)
	}

	out = h.must("", "up", "--file", file, "--yes")
	calls := strings.Join(h.world.calls, "\n")
	for _, want := range []string{"limactl start --name=myapp", "--cpus 6", "--memory 12", "50-rust.toml", "lima-myapp ghq get 'github.com/me/myapp'", "attach ssh myapp cd '/home/me.linux/code/github.com/me/myapp' && 'bash' '-ec' 'make deps\nmake db\n'", "systemctl start limabean-network"} {
		if !strings.Contains(calls, want) {
			t.Errorf("calls missing %q:\n%s", want, calls)
		}
	}
	if got := h.read("secrets.myapp.env"); !strings.Contains(got, "NPM_TOKEN=op://Work/npm/token") || strings.Contains(got, "SHARED") {
		t.Errorf("the VM's own secrets: %q", got)
	}
	if !strings.Contains(out, "http://localhost:3000") || !strings.Contains(out, "limabean code myapp /home/me.linux/code/github.com/me/myapp") {
		t.Errorf("out:\n%s", out)
	}

	// Again: the VM is there, so it's only started.
	h.world.vms["myapp"] = "Stopped"
	h.world.calls = nil
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "myapp is up") || h.world.called("limactl start --name=") || !h.world.called("limactl start --tty=false myapp") {
		t.Errorf("up again:\n%s\n%v", out, h.world.calls)
	}
}

func TestUpFindsFileAndSetupFails(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	dir := filepath.Join(t.TempDir(), "Plain Dir")
	sub := filepath.Join(dir, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".limabean.yaml"), []byte("setup: exit 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	h.world.attachErr = errors.New("exit status 1")
	out, err := h.run("", "up", "--yes")
	if err == nil || !strings.Contains(err.Error(), "setup failed") || !strings.Contains(out, "No repository to clone") || h.world.vms["plain-dir"] != "Running" {
		t.Fatalf("up: %v\n%s", err, out)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.run("", "up"); err == nil || !strings.Contains(err.Error(), "no .limabean.yaml") {
		t.Errorf("no file: %v", err)
	}
	bad := filepath.Join(t.TempDir(), ".limabean.yaml")
	if err := os.WriteFile(bad, []byte("packs: [cobol]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.run("", "up", "--file", bad, "--yes"); err == nil || !strings.Contains(err.Error(), "unknown pack") {
		t.Errorf("unknown pack: %v", err)
	}
	if _, err := h.run("", "up", "--file", filepath.Join(dir, "nope.yaml")); err == nil {
		t.Error("a missing file should fail")
	}
}
