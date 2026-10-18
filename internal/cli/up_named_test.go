package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/ui"
)

// up <name> makes or starts a VM without a project file, as new and start
// did.
func TestUpNamed(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	chdirTemp(t)

	out := h.must("", "up", "dev", "--with", "go", "--cpus", "3", "--memory", "5", "--disk", "20", "--ttl", "2h")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(out, "dev is ready") || !strings.Contains(calls, "limactl start --name=dev") || !strings.Contains(calls, "50-go.toml") ||
		!strings.Contains(calls, "--cpus 3") || !strings.Contains(calls, "--memory 5") || !strings.Contains(calls, "--disk 20") {
		t.Fatalf("create:\n%s\n%s", out, calls)
	}
	if !strings.Contains(out, "dev is deleted at") {
		t.Errorf("--ttl:\n%s", out)
	}

	// It exists: up starts it, or says it's running.
	if out := h.must("", "up", "dev"); !strings.Contains(out, "dev is already running") {
		t.Errorf("running:\n%s", out)
	}
	h.world.vms["dev"] = "Stopped"
	if out := h.must("", "up", "dev"); !strings.Contains(out, "Started dev") || h.world.vms["dev"] != "Running" {
		t.Errorf("stopped:\n%s", out)
	}

	// Choices for a new VM don't silently do nothing to an existing one.
	for _, args := range [][]string{{"up", "dev", "--with", "rust"}, {"up", "dev", "scratch"}} {
		if _, err := h.run("", args...); err == nil || !strings.Contains(err.Error(), "dev already exists") {
			t.Errorf("%v: %v", args, err)
		}
	}
	for args, want := range map[string]string{
		"up --setup":           "no .limabean.yaml here",
		"up":                   "name a VM to start or create",
		"up bad_name":          "VM name",
		"up other --disk lots": "a size in GiB",
		"up other --disk 0":    "a size in GiB",
		"down":                 "name the VM: limabean down <name>",
		"down --dry-run":       "--preview needs --idle",
		"down --idle 30s":      "a minute or more",
		"down nope":            "nope",
	} {
		if _, err := h.run("", strings.Fields(args)...); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", args, err, want)
		}
	}

	if out := h.must("", "down", "dev"); !strings.Contains(out, "Stopped dev") {
		t.Errorf("down dev:\n%s", out)
	}
	if out := h.must("", "down", "dev"); !strings.Contains(out, "already stopped") {
		t.Errorf("down dev again:\n%s", out)
	}
	if out := h.must("", "down", "--idle", "1h", "--dry-run"); out == "" {
		t.Error("down --idle printed nothing")
	}
}

// Without a name or a project file, a terminal gets new's form.
func TestUpForm(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	chdirTemp(t)
	out, err := h.runForms([][]string{{"formed", ui.Enter, ui.Enter}, {"n"}}, "up")
	if err != nil || !strings.Contains(out, "formed is ready") {
		t.Fatalf("%v\n%s", err, out)
	}
	h.world.vms["other"] = "Running"
	if out, err := h.runTTY([]string{ui.Down, ui.Enter}, "down"); err != nil || !strings.Contains(out, "Stopped") {
		t.Fatalf("down picks a VM: %v\n%s", err, out)
	}
}

// Inside a project, its own name means the file; any other is a VM of its own.
func TestUpInsideAProject(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "myapp", "packs: [node]\n")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(strings.TrimSuffix(file, "/.limabean.yaml")); err != nil {
		t.Fatal(err)
	}

	for args, want := range map[string]string{
		"up --yes --with go": "--with choose what a VM without a project file gets",
		"up myapp --cpus 4":  "change it there",
		"up myapp scratch":   "sets myapp's environment",
	} {
		if _, err := h.run("", strings.Fields(args)...); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", args, err, want)
		}
	}
	if out := h.must("", "up", "myapp", "--yes"); !strings.Contains(out, "myapp is ready") || !h.world.called("limactl start --name=myapp") {
		t.Fatalf("project by name:\n%s", out)
	}
	h.world.calls = nil
	if out := h.must("", "up", "side", "--with", "rust"); !strings.Contains(out, "side is ready") || !strings.Contains(strings.Join(h.world.calls, "\n"), "50-rust.toml") {
		t.Errorf("another VM:\n%s", out)
	}
	if out := h.must("", "down"); !strings.Contains(out, "Stopped myapp") || h.world.vms["side"] != "Running" {
		t.Errorf("down in the project:\n%s", out)
	}

	if got, _ := h.app(nil).completeUp(nil, []string{"x"}, ""); len(got) == 0 {
		t.Error("no environments completed")
	}
}

// chdirTemp moves into an empty directory, away from any project file.
func chdirTemp(t *testing.T) {
	t.Helper()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
