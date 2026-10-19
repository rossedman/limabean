package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/ui"
)

// checkout makes a git checkout with an origin, and a .limabean.yaml when
// project isn't empty, and moves into it.
func checkout(t *testing.T, project string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	chdirTemp(t)
	dir, _ := os.Getwd()
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "remote", "add", "origin", "https://github.com/acme/web")
	if project != "" {
		if err := os.WriteFile(".limabean.yaml", []byte(project), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func exitOf(err error) int {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	return 1
}

func TestRemote(t *testing.T) {
	h := newHarness(t)
	dir := checkout(t, "name: web\n")
	h.world.ghq = "/home/me.linux/code/github.com/acme/web\n"

	// No VM yet.
	if _, err := h.run("", "remote"); err == nil || exitOf(err) != exitNotFound {
		t.Fatalf("without a VM: %v", err)
	}
	h.world.vms["web"] = "Stopped"

	// Added: started first, fetching branches only.
	out := h.must("", "remote")
	want := "lima-web:/home/me.linux/code/github.com/acme/web"
	if got := gitIn(t, dir, "config", "--get", "remote.limabean.url"); got != want || !strings.Contains(out, "Added limabean") ||
		!strings.Contains(out, "git fetch limabean") || !h.world.called("limactl start") {
		t.Fatalf("added %q:\n%s", got, out)
	}
	if got := gitIn(t, dir, "config", "remote.limabean.tagOpt"); got != "--no-tags" {
		t.Errorf("tagOpt = %q", got)
	}

	// Again: nothing to do. From a subdirectory too.
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir("sub")
	if out := h.must("", "remote"); !strings.Contains(out, "already a remote") {
		t.Errorf("again:\n%s", out)
	}
	t.Chdir(dir)

	// After a rebuild the clone moved: pointed at the new one.
	h.world.ghq = "/home/me.linux/src/github.com/acme/web\n"
	if out := h.must("", "remote"); !strings.Contains(out, "Pointed limabean") ||
		gitIn(t, dir, "config", "--get", "remote.limabean.url") != "lima-web:/home/me.linux/src/github.com/acme/web" {
		t.Errorf("moved:\n%s", out)
	}

	// Another name, another VM.
	h.world.vms["other"] = "Running"
	h.must("", "remote", "other", "--name", "other")
	if got := gitIn(t, dir, "config", "--get", "remote.other.url"); got != "lima-other:/home/me.linux/src/github.com/acme/web" {
		t.Errorf("other = %q", got)
	}

	// Removed; removing again is fine.
	if out := h.must("", "remote", "--rm"); !strings.Contains(out, "Removed limabean") {
		t.Errorf("rm:\n%s", out)
	}
	if out := h.must("", "remote", "--rm"); !strings.Contains(out, "no remote named limabean") {
		t.Errorf("rm again:\n%s", out)
	}
}

func TestRemoteRefusals(t *testing.T) {
	h := newHarness(t)
	h.world.vms["web"] = "Running"

	// Not in a checkout.
	chdirTemp(t)
	if _, err := h.run("", "remote", "web"); err == nil || exitOf(err) != exitNotFound {
		t.Errorf("outside a checkout: %v", err)
	}

	dir := checkout(t, "name: web\n")
	// A name that would be a flag.
	if _, err := h.run("", "remote", "--name=--force"); err == nil || exitOf(err) != exitUsage {
		t.Errorf("--name=--force: %v", err)
	}

	// The clone's path comes from the VM: anything but a plain absolute
	// path stays out of the Mac's git config.
	for _, bad := range []string{"-oProxyCommand=sh/github.com/acme/web", "/home/me/code/github.com/../../acme/web", "/home/me/code/x;y/github.com/acme/web"} {
		h.world.ghq = bad + "\n"
		if _, err := h.run("", "remote"); err == nil || !strings.Contains(err.Error(), "won't put in a git remote") {
			t.Errorf("clone %q: %v", bad, err)
		}
	}
	if out, _ := exec.Command("git", "-C", dir, "remote").Output(); strings.Contains(string(out), "limabean") {
		t.Errorf("a remote was added: %s", out)
	}

	// A remote of that name that isn't a VM's is left alone.
	h.world.ghq = "/home/me/code/github.com/acme/web\n"
	gitIn(t, dir, "remote", "add", "limabean", "git@github.com:acme/fork")
	if _, err := h.run("", "remote"); err == nil || exitOf(err) != exitConflict {
		t.Errorf("taken name: %v", err)
	}
	if _, err := h.run("", "remote", "--rm"); err == nil || exitOf(err) != exitConflict {
		t.Errorf("rm of someone else's remote: %v", err)
	}
	if got := gitIn(t, dir, "config", "--get", "remote.limabean.url"); got != "git@github.com:acme/fork" {
		t.Errorf("the remote changed: %q", got)
	}

	// Never over MCP, which may not run anything on the Mac.
	a := h.app(ui.NewPlain(os.Stderr))
	a.mcpMode = true
	if _, err := a.runCLI(context.Background(), "remote"); err == nil || !strings.Contains(err.Error(), "isn't available over MCP") {
		t.Errorf("over MCP: %v", err)
	}
}
