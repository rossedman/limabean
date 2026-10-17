// Package templates gives limactl a directory of templates to read.
//
// The templates are embedded in the binary. Lima needs real files, and it
// resolves `base:` and `file:` references relative to each template, so the
// whole tree is written once to ~/.cache/limabean/templates/<hash>/, named
// after a hash of its contents: a binary with different templates gets a
// fresh copy, and an unchanged one reuses it.
package templates

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Dirs locates templates.
type Dirs struct {
	// Override is a templates directory to use instead of the embedded copy
	// (--templates, for working on the templates themselves).
	Override string
	// User holds your own environments (~/.config/limabean/templates). Each
	// <env>.yaml there can use the built-in layers: base: [./layer-base.yaml].
	User string
	// Cache is where the embedded copy is written (~/.cache/limabean/templates).
	Cache string
}

// Root returns the directory limactl should read templates from.
func (d Dirs) Root(embedded fs.FS) (string, error) {
	if d.Override != "" {
		if _, err := os.Stat(filepath.Join(d.Override, "layer-base.yaml")); err != nil {
			return "", fmt.Errorf("%s doesn't look like limabean's templates directory (no layer-base.yaml)", d.Override)
		}
		return d.Override, nil
	}
	sub, err := fs.Sub(embedded, "templates")
	if err != nil {
		return "", err
	}
	sum, err := hashFS(sub)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d.Cache, sum[:16])
	if _, err := os.Stat(filepath.Join(dir, ".complete")); err == nil {
		return dir, d.copyUser(dir)
	}
	tmp, err := os.MkdirTemp(d.Cache, ".tmp-")
	if err != nil {
		if err := os.MkdirAll(d.Cache, 0o755); err != nil {
			return "", err
		}
		if tmp, err = os.MkdirTemp(d.Cache, ".tmp-"); err != nil {
			return "", err
		}
	}
	if err := os.CopyFS(tmp, sub); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".complete"), nil, 0o644); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		os.RemoveAll(tmp)
		if _, statErr := os.Stat(filepath.Join(dir, ".complete")); statErr != nil {
			return "", err // nobody else finished it either
		}
	}
	return dir, d.copyUser(dir)
}

// copyUser copies your own environments next to the built-in layers, so
// their relative base: and file: references resolve. Built-in files win.
func (d Dirs) copyUser(root string) error {
	if d.User == "" {
		return nil
	}
	entries, err := os.ReadDir(d.User)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || strings.HasPrefix(e.Name(), "layer-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(d.User, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "user-"+e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Envs lists the environments in a templates root: built-in ones by name,
// and your own as written in your templates directory.
func Envs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var envs []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || e.IsDir() || strings.HasPrefix(name, "layer-") {
			continue
		}
		envs = append(envs, strings.TrimPrefix(name, "user-"))
	}
	slices.Sort(envs)
	return slices.Compact(envs), nil
}

// Path returns the template file for an environment. Your own environment
// replaces a built-in one with the same name.
func Path(root, env string) (string, error) {
	for _, name := range []string{"user-" + env + ".yaml", env + ".yaml"} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	envs, _ := Envs(root)
	return "", fmt.Errorf("unknown environment %q (available: %s)", env, strings.Join(envs, ", "))
}

func hashFS(fsys fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(data))
		h.Write(data)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}
