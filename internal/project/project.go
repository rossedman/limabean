// Package project reads .limabean.yaml, the file a repository keeps to say
// what VM it needs, for "limabean up".
//
// The file comes with the repository, so it's untrusted: it can name
// secrets but never say where their values come from (a cmd: reference
// would run on the Mac), and limabean shows what it asks for before
// creating anything.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FileName is the project file's name; FileNames are the names it's found
// by.
const FileName = ".limabean.yaml"

var FileNames = []string{FileName, ".limabean.yml"}

// ErrNotFound is Find's error when there's no project file.
var ErrNotFound = errors.New("no " + FileName + " (or .yml) here or in a parent directory")

// File is a .limabean.yaml.
type File struct {
	Name  string   `yaml:"name"`  // the VM's name; default the directory's
	Env   string   `yaml:"env"`   // environment template; default scratch
	Repo  string   `yaml:"repo"`  // what to clone; default the checkout's origin
	Packs []string `yaml:"packs"` // the VM's packs besides core; PACKS doesn't apply
	// Tools are the project's own mise tools, tool: version, and Apt its
	// own Ubuntu packages: what a pack would add, without one.
	Tools   map[string]string `yaml:"tools"`
	Apt     []string          `yaml:"apt"`
	Without []string          `yaml:"without"` // no longer used: accepted so older files still load
	CPUs    int               `yaml:"cpus"`
	Memory  int               `yaml:"memory"` // GiB
	Disk    int               `yaml:"disk"`   // GiB of the system disk
	Network struct {
		Allow  []string `yaml:"allow"`
		Locked bool     `yaml:"locked"`
	} `yaml:"network"`
	Secrets []string `yaml:"secrets"` // names of secrets the VM needs
	Ports   []int    `yaml:"ports"`   // what the project serves, to show after up
	Setup   string   `yaml:"setup"`   // shell commands run in the clone once, after creating
	Start   string   `yaml:"start"`   // shell commands run in the clone, in the background, after every up
	// Vars are plain variables for the VM's shells, setup and start;
	// secrets go in Secrets instead.
	Vars   map[string]string `yaml:"vars"`
	Claude Claude            `yaml:"claude"` // plugins for Claude Code in the VM
}

// Claude is what Claude Code in the VM gets: plugins, from their
// marketplaces, in its user settings. Skills, agents and the rest come
// inside plugins. All of it is fetched inside the VM.
type Claude struct {
	Marketplaces []Marketplace `yaml:"marketplaces"`
}

// Marketplace is a plugin marketplace and the plugins to turn on from it.
type Marketplace struct {
	Source  string   `yaml:"marketplace"` // GitHub owner/repo, or an https git URL
	Plugins []string `yaml:"plugins"`     // plugin@marketplace
}

// Name is the marketplace's name, as its plugins give it after the @.
func (m Marketplace) Name() string {
	if len(m.Plugins) == 0 {
		return ""
	}
	_, name, _ := strings.Cut(m.Plugins[0], "@")
	return name
}

// Empty reports whether the file asks for nothing for Claude.
func (c Claude) Empty() bool { return len(c.Marketplaces) == 0 }

// Plugins lists every plugin to turn on.
func (c Claude) Plugins() []string {
	var all []string
	for _, m := range c.Marketplaces {
		all = append(all, m.Plugins...)
	}
	return all
}

// Find looks for the project file in dir and its parents, and returns its
// path.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		var found []string
		for _, name := range FileNames {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				found = append(found, filepath.Join(dir, name))
			}
		}
		switch len(found) {
		case 1:
			return found[0], nil
		case 2:
			return "", fmt.Errorf("%s has both %s and %s; keep one", dir, FileNames[0], FileNames[1])
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w (see: limabean up --help)", ErrNotFound)
		}
		dir = parent
	}
}

var (
	nameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)
	secretRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	invalid  = regexp.MustCompile(`[^a-z0-9-]+`)

	githubRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)
	gitURLRE = regexp.MustCompile(`^https://[A-Za-z0-9.-]+(:[0-9]+)?/[A-Za-z0-9._~/-]+$`)
	pluginRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*@([A-Za-z0-9][A-Za-z0-9._-]*)$`)

	// mise tools, with an optional backend: go, aqua:cli/cli, npm:@scope/pkg.
	toolRE    = regexp.MustCompile(`^[A-Za-z0-9@][A-Za-z0-9._@/:+-]*$`)
	versionRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:+-]*$`)
	aptRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]+$`) // Debian package names
)

// Load reads and checks a project file. Unknown keys are errors, so typos
// don't go unnoticed. The name defaults to the file's directory's.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b, path)
}

// Parse checks a project file's contents, as Load does for the file at
// path.
func Parse(b []byte, path string) (*File, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Name == "" {
		f.Name = DefaultName(filepath.Dir(path))
	}
	if err := f.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &f, nil
}

// check validates the file's names, packs, ports, secrets, vars and Claude
// settings, so nothing odd reaches a command line.
func (f *File) check() error {
	if !nameRE.MatchString(f.Name) {
		return fmt.Errorf("name %q: use letters, digits and dashes", f.Name)
	}
	if f.Env != "" && !nameRE.MatchString(f.Env) {
		return fmt.Errorf("env %q isn't an environment name", f.Env)
	}
	for _, p := range append(append([]string{}, f.Packs...), f.Without...) {
		if !nameRE.MatchString(p) {
			return fmt.Errorf("pack %q isn't a pack name", p)
		}
	}
	for _, s := range f.Secrets {
		if !secretRE.MatchString(s) {
			return fmt.Errorf("secrets: %q isn't a secret's name (only names go here; set values with limabean secrets set)", s)
		}
	}
	if f.CPUs < 0 || f.Memory < 0 || f.Disk < 0 {
		return errors.New("cpus, memory and disk must be positive")
	}
	for _, p := range f.Ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("port %d is out of range", p)
		}
	}
	if strings.HasPrefix(f.Repo, "-") {
		return fmt.Errorf("repo %q isn't a repository", f.Repo)
	}
	if err := f.checkVars(); err != nil {
		return err
	}
	if err := f.checkTools(); err != nil {
		return err
	}
	return f.Claude.check()
}

// check validates the marketplaces and plugins a project asks Claude to use:
// their sources and plugin@marketplace names.
func (c Claude) check() error {
	names := map[string]bool{}
	for _, m := range c.Marketplaces {
		if !githubRE.MatchString(m.Source) && (!gitURLRE.MatchString(m.Source) || strings.Contains(m.Source, "..")) {
			return fmt.Errorf("claude.marketplaces: %q isn't a GitHub owner/repo or an https git URL", m.Source)
		}
		if len(m.Plugins) == 0 {
			return fmt.Errorf("claude.marketplaces: list the plugins to turn on from %s, as plugin@marketplace", m.Source)
		}
		for _, p := range m.Plugins {
			sub := pluginRE.FindStringSubmatch(p)
			if sub == nil {
				return fmt.Errorf("claude.marketplaces: %q isn't plugin@marketplace", p)
			}
			if sub[1] != m.Name() {
				return fmt.Errorf("claude.marketplaces: %s and %s name different marketplaces; give each its own entry", m.Plugins[0], p)
			}
		}
		if names[m.Name()] {
			return fmt.Errorf("claude.marketplaces: two entries are marketplace %s", m.Name())
		}
		names[m.Name()] = true
	}
	return nil
}

// reservedEnv are variables a project file can't set: the shell and the
// dynamic linker read them before anything the project runs.
var reservedEnv = map[string]bool{"PATH": true, "HOME": true, "USER": true, "SHELL": true, "ENV": true, "BASH_ENV": true, "ZDOTDIR": true, "PROMPT_COMMAND": true, "IFS": true}

// checkVars validates the file's vars: their names and values, and that none
// is reserved or also a secret.
func (f *File) checkVars() error {
	if len(f.Vars) > 200 {
		return errors.New("vars: at most 200 variables")
	}
	for name, value := range f.Vars {
		switch {
		case !secretRE.MatchString(name):
			return fmt.Errorf("vars: %q isn't a variable name", name)
		case reservedEnv[name] || strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_"):
			return fmt.Errorf("vars: %s is set by the VM, not a project file", name)
		case slices.Contains(f.Secrets, name):
			return fmt.Errorf("vars: %s is also a secret; keep it under secrets only", name)
		case strings.ContainsAny(value, "\x00\n\r") || len(value) > 4096:
			return fmt.Errorf("vars: %s's value must be one line of at most 4096 characters", name)
		}
	}
	return nil
}

// checkTools keeps tools and apt to names and versions, which limabean
// writes into the VM's mise and apt lists; nothing else can get in.
func (f *File) checkTools() error {
	if len(f.Tools) > 100 || len(f.Apt) > 100 {
		return errors.New("tools and apt: at most 100 each")
	}
	for tool, version := range f.Tools {
		if !toolRE.MatchString(tool) || len(tool) > 200 {
			return fmt.Errorf("tools: %q isn't a mise tool, like terraform or aqua:owner/repo", tool)
		}
		if !versionRE.MatchString(version) || len(version) > 100 {
			return fmt.Errorf("tools: %s's version %q isn't a version, like 1.9.8", tool, version)
		}
	}
	for _, p := range f.Apt {
		if !aptRE.MatchString(p) || len(p) > 100 {
			return fmt.Errorf("apt: %q isn't an Ubuntu package name", p)
		}
	}
	return nil
}

// DefaultName turns a directory's name into a VM name.
func DefaultName(dir string) string {
	n := strings.Trim(invalid.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "-"), "-")
	if n == "" {
		return "project"
	}
	return n
}
