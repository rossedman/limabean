package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/project"
)

// Every recipe must load, and bring a VM up: its packs, presets and
// plugins all have to be real.
func TestRecipesBringVMsUp(t *testing.T) {
	names := recipeNames()
	if len(names) < 10 {
		t.Fatalf("recipes: %v", names)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("LIMABEAN_CODE_DISK", "")
			body, err := recipe(name)
			if err != nil {
				t.Fatal(err)
			}
			if d := recipeDescription(body); d == "" || !strings.HasSuffix(d, ".") || len(d) > 76 {
				t.Errorf("first line must be a short sentence: %q", d)
			}
			if !strings.Contains(string(body), "limabean recipes "+name+" && limabean up") {
				t.Errorf("no usage line for %s", name)
			}
			dir := filepath.Join(t.TempDir(), name)
			file := filepath.Join(dir, project.FileName)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, body, 0o644); err != nil {
				t.Fatal(err)
			}
			h.must("", "up", "--file", file, "--yes")
			if h.world.vms[name] != "Running" {
				t.Errorf("%s isn't running: %v", name, h.world.vms)
			}
		})
	}
}

func TestRecipesCmd(t *testing.T) {
	h := newHarness(t)
	out := h.must("", "recipes")
	if !strings.Contains(out, "go-service") || !strings.Contains(out, "A Go service with Postgres in Docker.") {
		t.Errorf("list:\n%s", out)
	}
	if out := h.must("", "recipes", "rust", "--print"); !strings.HasPrefix(out, "# A Rust crate or CLI.") {
		t.Errorf("print:\n%s", out)
	}
	if _, err := h.run("", "recipes", "cobol"); err == nil || !strings.Contains(err.Error(), "there are: claude-agent") {
		t.Errorf("unknown: %v", err)
	}

	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if out := h.must("", "recipes", "rust"); !strings.Contains(out, "from the rust recipe") {
		t.Errorf("write:\n%s", out)
	}
	if b, _ := os.ReadFile(project.FileName); !strings.HasPrefix(string(b), "# A Rust crate") {
		t.Errorf("written: %q", b)
	}
	if _, err := h.run("", "recipes", "go-service"); err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Errorf("overwrite without --overwrite: %v", err)
	}
	h.must("", "recipes", "go-service", "--overwrite")
	if b, _ := os.ReadFile(project.FileName); !strings.HasPrefix(string(b), "# A Go service") {
		t.Errorf("forced: %q", b)
	}

	// A .yml file is the one replaced, so there's never both.
	if err := os.Remove(project.FileName); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".limabean.yml", []byte("cpus: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.must("", "recipes", "minimal", "-f")
	if _, err := os.Stat(project.FileName); err == nil {
		t.Error("wrote .limabean.yaml beside .limabean.yml")
	}
	if b, _ := os.ReadFile(".limabean.yml"); !strings.HasPrefix(string(b), "# The smallest VM") {
		t.Errorf(".yml: %q", b)
	}

	if got, _ := completeRecipes(nil, nil, ""); len(got) != len(recipeNames()) {
		t.Errorf("completion: %v", got)
	}
	if got, _ := completeRecipes(nil, []string{"rust"}, ""); got != nil {
		t.Errorf("completion after a name: %v", got)
	}
}
