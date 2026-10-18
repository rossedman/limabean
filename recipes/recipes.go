// Package recipes holds starter .limabean.yaml files, one per kind of VM.
// Each file's first comment line describes it; `limabean recipes` lists
// them and writes one into a repository.
package recipes

import "embed"

// Files are the recipes, named <recipe>.yaml.
//
//go:embed *.yaml
var Files embed.FS
