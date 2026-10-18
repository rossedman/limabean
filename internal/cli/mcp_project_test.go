package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPProjectTools(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	dir := filepath.Dir(projectFile(t, "proj", "packs: [go]\nsetup: make deps\nstart: make serve\nenvironment:\n  MODE: dev\n"))
	// The origin comes from .git/config, read as a file: git never runs.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]\n\tbare = false\n[remote \"origin\"]\n\turl = git@github.com:me/proj.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.world.ghq = "/home/me.linux/code/github.com/me/proj\n"
	cs, _ := h.mcpClient(false)

	out, isErr := call(t, cs, "project_plan", map[string]any{"dir": dir})
	if isErr || !strings.Contains(out, "clone: github.com/me/proj") || !strings.Contains(out, "make deps") || !strings.Contains(out, "Nothing created") || len(h.world.vms) != 0 {
		t.Fatalf("plan: %v\n%s", isErr, out)
	}

	// up: created, and setup runs without a terminal, its output in the reply.
	out, isErr = call(t, cs, "project_up", map[string]any{"dir": dir})
	calls := strings.Join(h.world.calls, "\n")
	if isErr || h.world.vms["proj"] != "Running" || !strings.Contains(calls, "ghq get 'github.com/me/proj'") ||
		!strings.Contains(calls, "'bash' '-ec' 'make deps'; } 2>&1 | tee") || !strings.Contains(h.world.sshStdin["proj.start"], "make serve") {
		t.Fatalf("up: %v\n%s\n%s", isErr, out, calls)
	}

	out, isErr = call(t, cs, "project_status", map[string]any{"dir": dir})
	var e psEntry
	if isErr || json.Unmarshal([]byte(out), &e) != nil || e.Status != "Running" || e.Name != "proj" {
		t.Fatalf("status: %v\n%s", isErr, out)
	}

	h.world.logText = "serving on :8080\n"
	if out, isErr := call(t, cs, "project_logs", map[string]any{"dir": dir, "log": "start", "lines": 5000}); isErr || !strings.Contains(out, "serving") ||
		!h.world.called(`ssh -F /lima/proj/ssh.config -o BatchMode=yes lima-proj tail -n 2000 "$HOME/.local/state/limabean/start.log"`) {
		t.Fatalf("logs: %v\n%s", isErr, out)
	}
	if _, isErr := call(t, cs, "project_logs", map[string]any{"dir": dir, "log": "secrets"}); !isErr {
		t.Error("an unknown log should fail")
	}
	if out, isErr := call(t, cs, "project_logs", map[string]any{"dir": dir}); isErr || !h.world.called(`ssh -F /lima/proj/ssh.config -o BatchMode=yes lima-proj sudo -n tail -n 100 "/var/log/cloud-init-output.log"`) {
		t.Errorf("provision log: %v\n%s", isErr, out)
	}

	// A change that needs a rebuild is refused without --allow-destructive.
	if err := os.WriteFile(filepath.Join(dir, ".limabean.yaml"), []byte("packs: [go, rust]\nstart: make serve\nenvironment:\n  MODE: dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, isErr := call(t, cs, "project_up", map[string]any{"dir": dir}); !isErr || !strings.Contains(out, "--allow-destructive") || h.world.called("limactl delete") {
		t.Fatalf("rebuild without --allow-destructive: %v\n%s", isErr, out)
	}

	if out, isErr := call(t, cs, "project_down", map[string]any{"dir": dir}); isErr || h.world.vms["proj"] != "Stopped" {
		t.Fatalf("down: %v\n%s", isErr, out)
	}

	// Only a directory's own project file is read.
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, ".git", "config"), filepath.Join(link, ".limabean.yaml")); err != nil {
		t.Fatal(err)
	}
	both := filepath.Dir(projectFile(t, "both", ""))
	if err := os.WriteFile(filepath.Join(both, ".limabean.yml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for d, want := range map[string]string{
		"relative/path":               "absolute path",
		filepath.Join(dir, "missing"): "isn't a directory",
		sub:                           "has no .limabean.yaml", // no looking in parents
		link:                          "has no .limabean.yaml", // a symlink isn't followed
		both:                          "has both",
	} {
		if out, isErr := call(t, cs, "project_status", map[string]any{"dir": d}); !isErr || !strings.Contains(out, want) {
			t.Errorf("%s: %v %s", d, isErr, out)
		}
	}
}

func TestMCPProjectRebuildAllowed(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "big", "packs: [go]\n")
	cs, _ := h.mcpClient(true)
	if _, isErr := call(t, cs, "project_up", map[string]any{"dir": filepath.Dir(file)}); isErr {
		t.Fatal("create")
	}
	rewrite(t, file, "packs: [go, rust]\n")
	if out, isErr := call(t, cs, "project_up", map[string]any{"dir": filepath.Dir(file)}); isErr || !h.world.called("limactl delete --force big") {
		t.Fatalf("rebuild with --allow-destructive: %v\n%s", isErr, out)
	}
}

func TestOriginFromConfig(t *testing.T) {
	dir := t.TempDir()
	if originFromConfig(dir) != "" {
		t.Error("no .git")
	}
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	for config, want := range map[string]string{
		"[remote \"upstream\"]\n\turl = x/y\n":                            "",
		"[remote \"origin\"]\n  url=https://github.com/a/b.git\n":         "https://github.com/a/b.git",
		"[remote \"origin\"]\n\turl = --upload-pack=evil\n":               "",
		"[remote \"origin\"]\n\tfetch = x\n[branch \"main\"]\nurl = no\n": "",
	} {
		os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(config), 0o644)
		if got := originFromConfig(dir); got != want {
			t.Errorf("%q: %q, want %q", config, got, want)
		}
	}
}
