package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/project"
)

func TestInitProject(t *testing.T) {
	h := newHarness(t)
	chdirTemp(t)
	for name, body := range map[string]string{
		"go.mod":         "module x\n",
		"compose.yaml":   "services: {}\n",
		".tool-versions": "terraform 1.9.8\nshellcheck 0.11.0\nmytool 1.2.3\n",
		".env.example":   "GITHUB_TOKEN=\n",
	} {
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// --print: the file on stdout, nothing written. The real packs decide:
	// terraform is infra's, shellcheck is core's.
	out := h.must("", "init", "--project", "--print")
	for _, want := range []string{project.SchemaComment, "packs: [go, infra]\n", "\"mytool\": \"1.2.3\"", "# secrets: [GITHUB_TOKEN]", "setup: |\n  go mod download\n", "start: |\n  docker compose up -d\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("print is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"shellcheck":`) || strings.Contains(out, `"terraform":`) {
		t.Errorf("a pack's tool went under tools:\n%s", out)
	}
	if _, err := os.Stat(project.FileName); err == nil {
		t.Fatal("--print wrote the file")
	}

	// Without a terminal it writes, then up can use it.
	out = h.must("", "init", "--project")
	if !strings.Contains(out, "Wrote") || !strings.Contains(out, "go: go.mod") || !strings.Contains(out, "Uncomment secrets") {
		t.Fatalf("write:\n%s", out)
	}
	if out := h.must("", "up", "--preview"); !strings.Contains(out, "packs: core, go, infra") || !strings.Contains(out, "tools: mytool 1.2.3") {
		t.Errorf("up --preview:\n%s", out)
	}

	// It's never replaced by accident.
	_, err := h.run("", "init", "--project")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != exitConflict {
		t.Errorf("again: %v", err)
	}
	h.must("", "init", "--project", "--overwrite")

	// In a terminal it asks; no keeps what's there.
	if err := os.WriteFile(project.FileName, []byte("cpus: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := h.runTTY([]string{"n"}, "init", "--project", "--overwrite"); err != nil || !strings.Contains(out, "Nothing written") || !strings.Contains(out, "packs: [go, infra]") {
		t.Fatalf("declined: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(project.FileName); string(b) != "cpus: 2\n" {
		t.Errorf("declined but written: %q", b)
	}

	if _, err := h.run("", "init", "--print"); err == nil || !strings.Contains(err.Error(), "go with --project") {
		t.Errorf("--print alone: %v", err)
	}
}

func TestInitProjectNothingFound(t *testing.T) {
	h := newHarness(t)
	chdirTemp(t)
	out := h.must("", "init", "--project")
	if !strings.Contains(out, "Nothing here says what the VM needs") || !strings.Contains(out, "limabean recipes") {
		t.Errorf("empty:\n%s", out)
	}
	if _, err := project.Load(project.FileName); err != nil {
		t.Errorf("the file doesn't load: %v", err)
	}
}

// After setup, init points at --project in a repository without a file.
func TestInitHintsAtProject(t *testing.T) {
	h := newHarness(t)
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}
	if out := h.must("", "init"); !strings.Contains(out, "limabean init --project") {
		t.Errorf("init in a repository:\n%s", out)
	}
}
