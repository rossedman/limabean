package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needJQ(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("needs jq")
	}
}

// features/claude.sh runs for real with bash in a fake home, on every boot:
// it adds limabean's status line and its CLAUDE.md block, keeps what the
// user set, and changes nothing the second time.
func TestClaudeFeature(t *testing.T) {
	needJQ(t)
	home := t.TempDir()
	boot := func() {
		t.Helper()
		cmd := exec.Command("bash", "../../templates/features/claude.sh")
		cmd.Env = append(os.Environ(), "HOME="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("claude.sh: %v\n%s", err, out)
		}
	}
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(home, ".claude", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(home, ".claude", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	settings := func() map[string]any {
		t.Helper()
		var s map[string]any
		if err := json.Unmarshal([]byte(read("settings.json")), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// A new VM.
	boot()
	s := settings()
	if s["statusLine"].(map[string]any)["command"] != "limabean-statusline" ||
		s["env"].(map[string]any)["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "1" {
		t.Errorf("settings = %v", s)
	}
	if md := read("CLAUDE.md"); !strings.Contains(md, "limabean-info") || !strings.Contains(md, "limabean network allow") {
		t.Errorf("CLAUDE.md:\n%s", md)
	}

	// The user's own choices, and limabean's plugins, stay; the block is
	// replaced, not added again.
	write("settings.json", `{"statusLine": {"type": "command", "command": "mine"}, "env": {"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "0", "FOO": "bar"}, "enabledPlugins": {"x@y": true}}`)
	write("CLAUDE.md", read("CLAUDE.md")+"\nMy own notes.\n")
	boot()
	boot()
	s = settings()
	env := s["env"].(map[string]any)
	if s["statusLine"].(map[string]any)["command"] != "mine" || env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "0" ||
		env["FOO"] != "bar" || s["enabledPlugins"] == nil {
		t.Errorf("the user's settings changed: %v", s)
	}
	md := read("CLAUDE.md")
	if strings.Count(md, "This machine is a limabean VM") != 1 || !strings.Contains(md, "My own notes.") {
		t.Errorf("CLAUDE.md after two boots:\n%s", md)
	}

	// Settings that aren't a JSON object are left alone, and boot goes on.
	write("settings.json", "not json")
	boot()
	if read("settings.json") != "not json" {
		t.Error("invalid settings were rewritten")
	}
}

// limabean-statusline runs for real with sh, on what Claude Code sends it.
func TestStatusLine(t *testing.T) {
	needJQ(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "limabean")
	repo := filepath.Join(home, "code", "app")
	for _, d := range []string{dir, repo} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "vm.json"), []byte(`{"name": "app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(dir, "network.conf")
	gitIn(t, repo, "init", "-q", "-b", "agent/fix")
	run := func(conf, session string) string {
		t.Helper()
		if err := os.WriteFile(policy, []byte(conf), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", "../../templates/files/limabean-statusline")
		cmd.Env = append(os.Environ(), "HOME="+home, "LIMABEAN_NETWORK_CONF="+policy, "NO_COLOR=1")
		cmd.Stdin = strings.NewReader(session)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("statusline: %v", err)
		}
		return strings.TrimSpace(string(out))
	}
	session := `{"model": {"display_name": "Opus 5.5"}, "workspace": {"current_dir": "` + repo + `"}}`

	got := run("locked\nallow github.com\nallow registry.npmjs.org\nallow api.anthropic.com\n", session)
	for _, want := range []string{"app · only github.com +2, locked · agent/fix", "Opus 5.5"} {
		if !strings.Contains(got, want) {
			t.Errorf("locked: missing %q in %q", want, got)
		}
	}
	if got := run("allow github.com\n", session); !strings.Contains(got, "only github.com ·") || strings.Contains(got, "locked") {
		t.Errorf("one host: %q", got)
	}
	if got := run("learn\n", session); !strings.Contains(got, "learning") {
		t.Errorf("learning: %q", got)
	}
	// No policy, no session details: still a line.
	if got := run("", "{}"); !strings.HasPrefix(got, "app · network open") {
		t.Errorf("open: %q", got)
	}
}
