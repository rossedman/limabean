package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spf13/cobra"
)

// Every command limabean defines must be documented for someone who has
// never used it: a short line, a longer description and examples.
func TestEveryCommandIsDocumented(t *testing.T) {
	root := NewRootCmd(NewApp("test", fstest.MapFS{}))
	var walk func(*cobra.Command)
	count := 0
	walk = func(c *cobra.Command) {
		if c.Hidden || c.Name() == "help" {
			return
		}
		count++
		path := c.CommandPath()
		if strings.TrimSpace(c.Short) == "" {
			t.Errorf("%s: Short is empty", path)
		}
		if strings.TrimSpace(c.Long) == "" {
			t.Errorf("%s: Long is empty", path)
		}
		if strings.TrimSpace(c.Example) == "" {
			t.Errorf("%s: Example is empty", path)
		}
		if strings.HasSuffix(strings.TrimSpace(c.Short), ".") {
			t.Errorf("%s: Short shouldn't end with a period", path)
		}
		c.Flags().VisitAll(func(f *pflagFlag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Errorf("%s --%s: flag has no description", path, f.Name)
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	if count < 20 {
		t.Fatalf("only %d commands found; is the tree wired up?", count)
	}
}

// Each command is defined in its own file: no file in this package builds
// more than one cobra.Command.
func TestOneCommandPerFile(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		ast.Inspect(file, func(node ast.Node) bool {
			if lit, ok := node.(*ast.CompositeLit); ok {
				if sel, ok := lit.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Command" {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "cobra" {
						n++
					}
				}
			}
			return true
		})
		if n > 1 {
			t.Errorf("%s defines %d commands; put each in its own file", f, n)
		}
	}
}
