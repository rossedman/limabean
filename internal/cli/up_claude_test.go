package cli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClaude is a claude CLI that logs its arguments, and fails to install
// nope@* and to add a marketplace from broken/repo, as the real one does.
const fakeClaude = `#!/bin/sh
echo "$*" >> "$HOME/claude.log"
case "$*" in
  "plugin install nope@"*)
    echo '{"command":"install","outcome":"failed","message":"Plugin \"nope\" not found"}'
    echo 'Failed to install' >&2
    exit 1 ;;
  "plugin marketplace add broken/repo --json")
    echo 'fatal: repository not found' >&2
    exit 1 ;;
esac
echo '{"outcome":"ok"}'
`

// runClaudeScript runs claudeScript with python3, as up does in the VM,
// with HOME at home and bin first on PATH, and returns its last line and
// what claude was asked to do.
func runClaudeScript(t *testing.T, home, bin string, data map[string]any) (map[string]any, string) {
	t.Helper()
	b, _ := json.Marshal(data)
	cmd := exec.Command("python3", "-")
	cmd.Env = []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin"}
	cmd.Stdin = strings.NewReader("DATA = \"" + base64.StdEncoding.EncodeToString(b) + "\"\n" + claudeScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var res map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil {
		t.Fatalf("last line isn't JSON: %s", out)
	}
	log, _ := os.ReadFile(filepath.Join(home, "claude.log"))
	return res, string(log)
}

func TestClaudeScript(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	data := map[string]any{
		"marketplaces": map[string]string{"guardrails": "rossedman/guardrails", "team": "https://git.example.com/team/plugins.git", "gone": "broken/repo"},
		"plugins":      []string{"guardrails@guardrails", "review@team", "nope@team", "x@gone"},
	}

	// No claude CLI: settings only, and say so.
	home, bin := t.TempDir(), t.TempDir()
	res, _ := runClaudeScript(t, home, bin, data)
	if res["no_cli"] != true {
		t.Errorf("without claude: %v", res)
	}
	b, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(string(b), `"review@team": true`) || !strings.Contains(string(b), `"url": "https://git.example.com/team/plugins.git"`) {
		t.Errorf("settings.json:\n%s", b)
	}

	// With it: every marketplace added, every plugin installed, failures
	// reported with claude's own message.
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fakeClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	res, log := runClaudeScript(t, home, bin, data)
	for _, want := range []string{
		"plugin marketplace add rossedman/guardrails --json",
		"plugin marketplace add https://git.example.com/team/plugins.git --json",
		"plugin install guardrails@guardrails --json",
		"plugin install review@team --json",
	} {
		if !strings.Contains(log, want+"\n") {
			t.Errorf("claude not run as %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "x@gone") || strings.Contains(log, " -y") {
		t.Errorf("installed from a marketplace that failed, or accepted a command:\n%s", log)
	}
	failed, _ := res["failed"].([]any)
	got, _ := json.Marshal(failed)
	if len(failed) != 2 || !strings.Contains(string(got), `nope@team: Plugin \"nope\" not found`) ||
		!strings.Contains(string(got), "x@gone: fatal: repository not found") {
		t.Errorf("failed = %s", got)
	}
}
