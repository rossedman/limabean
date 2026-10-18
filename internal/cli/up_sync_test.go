package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projectFile writes a project file in a directory named after the VM and
// returns its path.
func projectFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, ".limabean.yaml")
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func rewrite(t *testing.T, file, body string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpAppliesChanges(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "app", "packs: [go]\ncpus: 2\nmemory: 4\n")
	h.must("", "up", "--file", file, "--yes")

	// Nothing changed: up only says so, and never asks.
	h.world.calls = nil
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "app is up to date") || h.world.called("limactl edit") {
		t.Fatalf("unchanged:\n%s\n%v", out, h.world.calls)
	}

	// A stopped VM is started.
	h.world.vms["app"] = "Stopped"
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "up to date") || h.world.vms["app"] != "Running" {
		t.Fatalf("stopped:\n%s", out)
	}

	// A new size: listed, then applied with --yes by stopping, editing, starting.
	rewrite(t, file, "packs: [go]\ncpus: 4\nmemory: 8\ndisk: 30\n")
	out, err := h.run("", "up", "--file", file)
	if err == nil || !strings.Contains(err.Error(), "--yes") || !strings.Contains(out, "cpus: 2 → 4") || !strings.Contains(out, "disk: 0 → 30 GiB") {
		t.Fatalf("size without --yes: %v\n%s", err, out)
	}
	h.world.calls = nil
	h.must("", "up", "--file", file, "--yes")
	if !h.world.called("limactl stop app") || !h.world.called("limactl edit --tty=false --cpus 4 --memory 8 --disk 30 app") || h.world.vms["app"] != "Running" {
		t.Fatalf("resize: %v", h.world.calls)
	}

	// A smaller disk can't be had; up says so and changes nothing.
	rewrite(t, file, "packs: [go]\ncpus: 4\nmemory: 8\ndisk: 20\n")
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "can't shrink") || !strings.Contains(out, "up to date") {
		t.Errorf("shrink:\n%s", out)
	}

	// Declining in a terminal leaves the VM as it is.
	rewrite(t, file, "packs: [go]\ncpus: 6\nmemory: 8\n")
	h.world.calls = nil
	if out, err := h.runTTY([]string{"n"}, "up", "--file", file); err != nil || !strings.Contains(out, "Left app as it is") || h.world.called("limactl edit") {
		t.Fatalf("declined: %v\n%s\n%v", err, out, h.world.calls)
	}

	// New packs: the VM is rebuilt, keeping its ~/code disk, with the new size too.
	rewrite(t, file, "packs: [go, rust]\ncpus: 6\nmemory: 8\n")
	h.world.calls = nil
	out = h.must("", "up", "--file", file, "--yes")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(out, "packs: core, go → core, go, rust (rebuild, keeping ~/code)") || !strings.Contains(calls, "limactl delete --force app") ||
		!strings.Contains(calls, "50-rust.toml") || !strings.Contains(calls, "--cpus 6") || !strings.Contains(calls, "limactl stop app") {
		t.Fatalf("rebuild:\n%s\n%s", out, calls)
	}
	if got := h.read("../state/limabean/app.packs"); got != "go,rust\n" {
		t.Errorf("packs saved: %q", got)
	}

	// --setup runs the setup again, in the clone.
	rewrite(t, file, "packs: [go, rust]\ncpus: 6\nmemory: 8\nrepo: me/app\nsetup: make deps\n")
	h.world.ghq = "/home/me.linux/code/github.com/me/app\n"
	h.world.calls = nil
	h.must("", "up", "--file", file, "--setup")
	if !h.world.called("attach ssh app cd '/home/me.linux/code/github.com/me/app' && 'bash' '-ec' 'make deps'") {
		t.Errorf("setup: %v", h.world.calls)
	}
}

func TestUpRebuildWithoutCodeDisk(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "plain", "packs: [go]\n")
	h.must("", "up", "--file", file, "--yes")
	rewrite(t, file, "packs: [node]\n")

	// Unpushed work and no separate disk: up won't rebuild.
	h.world.unsaved = "~/code/github.com/me/app: 1 unpushed commit\n"
	if _, err := h.run("", "up", "--file", file, "--yes"); err == nil || !strings.Contains(err.Error(), "no separate ~/code disk") || h.world.called("limactl delete") {
		t.Fatalf("unsaved: %v", err)
	}
	h.world.unsaved = ""
	h.must("", "up", "--file", file, "--yes")
	if !h.world.called("limactl delete --force plain") {
		t.Errorf("clean rebuild: %v", h.world.calls)
	}
}

// A project file may tighten a VM's network but never loosen it.
func TestUpNetworkOnlyTightens(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "net", "network:\n  allow: [github, npm]\n")
	h.must("", "up", "--file", file, "--yes")
	policy := func() string { return h.read("network/net/network.conf") }

	// More domains: not applied; up prints the command to run.
	rewrite(t, file, "network:\n  allow: [github, npm, python]\n")
	out := h.must("", "up", "--file", file)
	if !strings.Contains(out, "limabean network allow net pypi.org pythonhosted.org") || strings.Contains(policy(), "pypi.org") {
		t.Fatalf("widening:\n%s\n%s", out, policy())
	}

	// Fewer domains and a lock: applied.
	rewrite(t, file, "network:\n  allow: [github]\n  locked: true\n")
	h.world.calls = nil
	out = h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(out, "network: stop allowing npmjs.com, npmjs.org, yarnpkg.com") || !strings.Contains(policy(), "locked") ||
		strings.Contains(policy(), "npmjs") || !h.world.called("ssh -F /lima/net/ssh.config -o BatchMode=yes lima-net sudo -n systemctl start limabean-network.service") {
		t.Fatalf("tightening:\n%s\n%s\n%v", out, policy(), h.world.calls)
	}

	// An open network, or no lock: only the user can do that.
	rewrite(t, file, "packs: []\n")
	out = h.must("", "up", "--file", file)
	if !strings.Contains(out, "limabean network open net") || !strings.Contains(out, "limabean network unlock net") || !strings.Contains(policy(), "locked") {
		t.Fatalf("loosening:\n%s\n%s", out, policy())
	}

	// Entirely different domains: removing them all would open it, so
	// both steps are left to the user.
	rewrite(t, file, "network:\n  allow: [rust]\n  locked: true\n")
	out = h.must("", "up", "--file", file)
	if !strings.Contains(out, "limabean network allow net crates.io rust-lang.org") || !strings.Contains(out, "limabean network rm net") || !strings.Contains(policy(), "github.com") {
		t.Fatalf("disjoint:\n%s\n%s", out, policy())
	}
}

func TestUpNetworkOnOpenAndOldVMs(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "open", "")
	h.must("", "up", "--file", file, "--yes")
	rewrite(t, file, "network:\n  allow: [github]\n")
	if out := h.must("", "up", "--file", file, "--yes"); !strings.Contains(out, "network: only github") || !strings.Contains(h.read("network/open/network.conf"), "allow github.com") {
		t.Fatalf("open → allowlist:\n%s", out)
	}

	h.world.oldVMs["open"] = true
	rewrite(t, file, "network:\n  allow: [github]\n  locked: true\n")
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "limabean rebuild open") {
		t.Errorf("old VM:\n%s", out)
	}
}

func TestUpAddsSecrets(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "sec", "")
	h.must("", "up", "--file", file, "--yes")
	h.must("", "secrets", "set", "API_KEY", "plain-value", "--vm", "other", "--no-sync")
	rewrite(t, file, "secrets: [API_KEY, NOPE]\n")
	out := h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(out, "secret: add API_KEY") || !strings.Contains(out, "won't get them: NOPE") ||
		!strings.Contains(h.read("secrets.sec.env"), "API_KEY=plain-value") || !strings.Contains(h.world.sshStdin["sec"], "API_KEY") {
		t.Fatalf("secrets:\n%s\n%q", out, h.read("secrets.sec.env"))
	}
}

func TestUpSetupNeedsARepo(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "norepo", "")
	h.must("", "up", "--file", file, "--yes")
	if _, err := h.run("", "up", "--file", file, "--setup"); err == nil || !strings.Contains(err.Error(), "no repository") {
		t.Errorf("no repo: %v", err)
	}
	rewrite(t, file, "repo: me/norepo\n")
	h.world.ghq = "/home/me.linux/code/github.com/me/norepo\n"
	if out := h.must("", "up", "--file", file, "--setup"); !strings.Contains(out, "no setup to run") {
		t.Errorf("no setup:\n%s", out)
	}
}

func TestUpResizeFails(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "small", "cpus: 2\n")
	h.must("", "up", "--file", file, "--yes")
	rewrite(t, file, "cpus: 3\n")
	h.world.failStop = true
	if _, err := h.run("", "up", "--file", file, "--yes"); err == nil {
		t.Error("a failed stop should stop up")
	}
	h.world.failStop = false
	h.world.failNetwork = true
	rewrite(t, file, "network:\n  allow: [github]\n")
	if _, err := h.run("", "up", "--file", file, "--yes"); err == nil {
		t.Error("a policy that won't apply should fail up")
	}
}

func TestDown(t *testing.T) {
	h := newHarness(t)
	file := projectFile(t, "web", "")
	if out := h.must("", "down", "--file", file); !strings.Contains(out, "web has no VM yet") {
		t.Errorf("no VM:\n%s", out)
	}
	h.must("", "up", "--file", file, "--yes")
	if out := h.must("", "down", "--file", file); !strings.Contains(out, "Stopped web") || h.world.vms["web"] != "Stopped" {
		t.Fatalf("down:\n%s", out)
	}
	if out := h.must("", "down", "--file", file); !strings.Contains(out, "already stopped") {
		t.Errorf("again:\n%s", out)
	}

	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}
	h.must("", "up", "--yes")
	if h.must("", "down"); h.world.vms["web"] != "Stopped" {
		t.Error("down from the project's directory")
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.run("", "down"); err == nil || !strings.Contains(err.Error(), "no .limabean.yaml") {
		t.Errorf("no file: %v", err)
	}
}
