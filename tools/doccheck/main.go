// Command doccheck fails when a hand-written Go function has no doc comment
// or a package has no package comment. Generated and _test.go files are
// skipped. Run it as `go run ./tools/doccheck ./...`.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skipDirs are directory names the walk never enters.
var skipDirs = map[string]bool{
	".git":         true,
	"dist":         true,
	"node_modules": true,
	"site":         true,
}

// main runs the checker over the command-line arguments and exits with its
// status.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

// run checks every root named in args (default "./...") and writes problems
// to stdout. It returns the process exit code: 0 clean, 1 findings, 2 error.
func run(args []string, stdout io.Writer) int {
	if len(args) == 0 {
		args = []string{"./..."}
	}
	var problems []string
	for _, arg := range args {
		root := strings.TrimSuffix(strings.TrimSuffix(arg, "..."), "/")
		if root == "" {
			root = "."
		}
		found, err := check(root)
		if err != nil {
			_, _ = fmt.Fprintln(stdout, "doccheck:", err)
			return 2
		}
		problems = append(problems, found...)
	}
	for _, p := range problems {
		_, _ = fmt.Fprintln(stdout, p)
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// check walks root and returns one message per function without a doc
// comment and per package directory without a package comment.
func check(root string) ([]string, error) {
	var problems []string
	pkgDoc := map[string]bool{}  // dir -> some file has a package comment
	pkgSeen := map[string]bool{} // dir -> has at least one checked file
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		if ast.IsGenerated(f) {
			return nil
		}
		dir := filepath.Dir(path)
		pkgSeen[dir] = true
		if f.Doc != nil {
			pkgDoc[dir] = true
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Doc == nil {
				pos := fset.Position(fn.Pos())
				problems = append(problems, fmt.Sprintf("%s:%d: %s has no doc comment", path, pos.Line, fn.Name.Name))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for dir := range pkgSeen {
		if !pkgDoc[dir] {
			problems = append(problems, fmt.Sprintf("%s: package has no package comment", dir))
		}
	}
	sort.Strings(problems)
	return problems, nil
}
