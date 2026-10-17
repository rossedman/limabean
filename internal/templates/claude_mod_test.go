package templates

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every file of the Claude Code mod that it loads (not its tests, nor the
// types the engine writes beside it) is written into VMs by layer-base.yaml,
// so a new file can't be left out of them.
func TestClaudeModIsProvisioned(t *testing.T) {
	repo := filepath.Join("..", "..", "templates")
	base, err := os.ReadFile(filepath.Join(repo, "layer-base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	mod := filepath.Join(repo, "claude", "limabean")
	n := 0
	err = filepath.WalkDir(mod, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(mod, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "tests" || rel == ".claude-plugin/types" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".test.ts") || strings.HasSuffix(rel, ".test.tsx") || filepath.Base(rel) == ".gitignore" {
			return nil
		}
		n++
		want := "  path: /usr/local/share/limabean/claude/limabean/" + rel + "\n  file: ./claude/limabean/" + rel + "\n"
		if !strings.Contains(string(base), want) {
			t.Errorf("layer-base.yaml doesn't write %s into VMs; add:\n- mode: data\n%s", rel, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 5 {
		t.Errorf("found only %d mod files", n)
	}
}
