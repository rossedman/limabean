package cli

import (
	"encoding/base64"
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
	for _, want := range []string{"limactl start --name=myapp", "--cpus 6", "--memory 12", "50-rust.toml", "lima-myapp ghq get 'github.com/me/myapp'", "cd '/home/me.linux/code/github.com/me/myapp' && 'bash' '-ec' 'make deps\nmake db\n'; } 2>&1 | tee \"$HOME/.local/state/limabean/setup.log\"", "systemctl start limabean-network"} {
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
	if out := h.must("", "up", "--file", file); !strings.Contains(out, "myapp is up") || h.world.called("limactl start --name=") || !h.world.called("limactl start --tty=false --timeout=30m0s myapp") {
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

func TestUpClaude(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	dir := filepath.Join(t.TempDir(), "agent")
	file := filepath.Join(dir, ".limabean.yaml")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`claude:
  marketplaces:
    - marketplace: rossedman/guardrails
      plugins: [guardrails@guardrails]
    - marketplace: https://git.example.com/team/plugins.git
      plugins: [review@team, deploy@team]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := h.run("", "up", "--file", file)
	for _, want := range []string{
		"claude plugins from rossedman/guardrails: guardrails@guardrails",
		"claude plugins from https://git.example.com/team/plugins.git: review@team, deploy@team",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan missing %q:\n%s", want, out)
		}
	}

	out = h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(out, "Claude plugins for agent: guardrails@guardrails, review@team, deploy@team") {
		t.Errorf("out:\n%s", out)
	}
	// The list reaches the VM as base64 JSON on stdin, never on a command line.
	in := h.world.sshStdin["agent"]
	line, _, _ := strings.Cut(in, "\n")
	data, err := base64.StdEncoding.DecodeString(strings.Trim(strings.TrimPrefix(line, "DATA = "), `"`))
	if err != nil || !strings.Contains(string(data), `"team":"https://git.example.com/team/plugins.git"`) ||
		!strings.Contains(string(data), `"guardrails@guardrails"`) || !strings.Contains(in, "enabledPlugins") {
		t.Errorf("stdin: %q (%s, %v)", in, data, err)
	}
	if strings.Contains(strings.Join(h.world.calls, "\n"), "rossedman/guardrails") {
		t.Errorf("the list reached a command line: %v", h.world.calls)
	}

	// Without the claude CLI in the VM, they are only turned on.
	h.world.vms = map[string]string{}
	h.world.claudeOut = `{"no_cli": true}`
	if out := h.must("", "up", "--file", file, "--yes"); !strings.Contains(out, "/plugin in a Claude session installs them") {
		t.Errorf("no claude CLI:\n%s", out)
	}
	// A plugin that didn't install is a warning of its own.
	h.world.vms = map[string]string{}
	h.world.claudeOut = "some progress\n" + `{"failed": ["deploy@team: Plugin \"deploy\" not found in marketplace \"team\""]}`
	out = h.must("", "up", "--file", file, "--yes")
	if !strings.Contains(out, `Claude plugin not installed: deploy@team: Plugin "deploy" not found`) || !strings.Contains(out, "Claude plugins for agent") {
		t.Errorf("one failed:\n%s", out)
	}
	h.world.vms = map[string]string{}
	h.world.claudeOut = `{"failed": ["guardrails@guardrails: x", "review@team: x", "deploy@team: x"]}`
	if out := h.must("", "up", "--file", file, "--yes"); strings.Contains(out, "Claude plugins for agent") {
		t.Errorf("all failed, but reported as added:\n%s", out)
	}

	// A failure is a warning: the VM is still there and usable.
	h.world.vms = map[string]string{}
	h.world.claudeErr = true
	if out := h.must("", "up", "--file", file, "--yes"); !strings.Contains(out, "Claude plugins not added") {
		t.Errorf("failure:\n%s", out)
	}
}

// A project's VM gets the packs its file lists, whatever PACKS says, so
// everyone gets the same VM.
func TestUpPacksAreExact(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	t.Setenv("LIMABEAN_PACKS", "go,node,kubernetes")
	dir := filepath.Join(t.TempDir(), "exact")
	file := filepath.Join(dir, ".limabean.yaml")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("packs: [rust]\nwithout: [kubernetes]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := h.must("", "up", "--file", file, "--yes")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(out, "packs: core, rust") || !strings.Contains(calls, "50-rust.toml") || strings.Contains(calls, "50-go.toml") || strings.Contains(calls, "50-node.toml") {
		t.Errorf("packs:\n%s\n%s", out, calls)
	}
	if !strings.Contains(out, "without isn't needed any more") {
		t.Errorf("no warning about without:\n%s", out)
	}
}
