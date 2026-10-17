package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func embedded() fstest.MapFS {
	return fstest.MapFS{
		"templates/layer-base.yaml":    {Data: []byte("base: [./layer-os.yaml]\n")},
		"templates/layer-os.yaml":      {Data: []byte("vmType: vz\n")},
		"templates/scratch.yaml":       {Data: []byte("base: [./layer-base.yaml]\n")},
		"templates/features/git.sh":    {Data: []byte("#!/bin/bash\n")},
		"templates/files/editorconfig": {Data: []byte("root = true\n")},
	}
}

func TestRootWritesCacheOnceAndReuses(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "templates")
	d := Dirs{Cache: cache}
	root, err := d.Root(embedded())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"layer-base.yaml", "scratch.yaml", "features/git.sh", "files/editorconfig"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	again, err := d.Root(embedded())
	if err != nil || again != root {
		t.Fatalf("second Root = %s, %v; want %s", again, err, root)
	}
	changed := embedded()
	changed["templates/scratch.yaml"] = &fstest.MapFile{Data: []byte("changed\n")}
	other, err := d.Root(changed)
	if err != nil || other == root {
		t.Fatalf("changed templates should get a new dir, got %s (%v)", other, err)
	}
}

func TestUserEnvironments(t *testing.T) {
	user := t.TempDir()
	if err := os.WriteFile(filepath.Join(user, "big.yaml"), []byte("base: [./layer-base.yaml]\ncpus: 8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(user, "scratch.yaml"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := Dirs{Cache: t.TempDir(), User: user}.Root(embedded())
	if err != nil {
		t.Fatal(err)
	}
	envs, _ := Envs(root)
	if strings.Join(envs, ",") != "big,scratch" {
		t.Fatalf("Envs = %v", envs)
	}
	p, err := Path(root, "scratch")
	if err != nil || filepath.Base(p) != "user-scratch.yaml" {
		t.Fatalf("Path(scratch) = %s, %v; your own should win", p, err)
	}
	if _, err := Path(root, "nope"); err == nil || !strings.Contains(err.Error(), "big, scratch") {
		t.Fatalf("unknown env error = %v", err)
	}
}

func TestOverride(t *testing.T) {
	dir := t.TempDir()
	if _, err := (Dirs{Override: dir}).Root(embedded()); err == nil {
		t.Fatal("want an error for a directory without layer-base.yaml")
	}
	if err := os.WriteFile(filepath.Join(dir, "layer-base.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := (Dirs{Override: dir}).Root(embedded()); err != nil || got != dir {
		t.Fatalf("Root = %s, %v", got, err)
	}
}

func TestRealTemplatesEmbed(t *testing.T) {
	// The repository's own templates must include every layer and env.
	root, err := Dirs{Cache: t.TempDir()}.Root(os.DirFS(filepath.Join("..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"layer-os.yaml", "layer-base.yaml", "scratch.yaml", "features/mise.sh", "files/env.sh"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("embedded copy is missing %s", f)
		}
	}
}
