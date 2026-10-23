// Command gendocs writes one Markdown page per limabean command into
// docs/reference/commands, from the commands' own help, so the reference
// never drifts from the CLI. Run it with `make docs`; CI fails when the
// committed pages are stale.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/rossedman/limabean"
	"github.com/rossedman/limabean/internal/cli"
)

// main exits with run's status.
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run writes the pages into the directory named by args (default
// docs/reference/commands), replacing the pages already there.
func run(args []string, stderr io.Writer) int {
	dir := filepath.Join("docs", "reference", "commands")
	if len(args) > 0 {
		dir = args[0]
	}
	if err := generate(dir); err != nil {
		_, _ = fmt.Fprintln(stderr, "gendocs:", err)
		return 1
	}
	return 0
}

// generate empties dir of Markdown pages and writes a page for every
// command that isn't hidden.
func generate(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	// A fixed version: the pages mustn't change with each build.
	root := cli.NewRootCmd(cli.NewApp("dev", limabean.Templates))
	root.DisableAutoGenTag = true
	return doc.GenMarkdownTree(root, dir)
}
