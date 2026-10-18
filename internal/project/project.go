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
	"strings"

	"go.yaml.in/yaml/v3"
)

// FileName is the project file's name; FileNames are the names it's found
// by.
const FileName = ".limabean.yaml"

var FileNames = []string{FileName, ".limabean.yml"}

// File is a .limabean.yaml.
type File struct {
	Name    string   `yaml:"name"`    // the VM's name; default the directory's
	Env     string   `yaml:"env"`     // environment template; default scratch
	Repo    string   `yaml:"repo"`    // what to clone; default the checkout's origin
	Packs   []string `yaml:"packs"`   // added to the default packs
	Without []string `yaml:"without"` // default packs to leave out
	CPUs    int      `yaml:"cpus"`
	Memory  int      `yaml:"memory"` // GiB
	Disk    int      `yaml:"disk"`   // GiB of the system disk
	Network struct {
		Allow  []string `yaml:"allow"`
		Locked bool     `yaml:"locked"`
	} `yaml:"network"`
	Secrets []string `yaml:"secrets"` // names of secrets the VM needs
	Ports   []int    `yaml:"ports"`   // what the project serves, to show after up
	Setup   string   `yaml:"setup"`   // shell commands run in the clone once, after creating
	Claude  Claude   `yaml:"claude"`  // plugins and skills for Claude Code in the VM
}

// Claude is what Claude Code in the VM gets: marketplaces and plugins in
// its user settings, and skills in ~/.claude/skills. All of it is fetched
// inside the VM.
type Claude struct {
	Marketplaces map[string]string `yaml:"marketplaces"` // name → GitHub owner/repo or https git URL
	Plugins      []string          `yaml:"plugins"`      // plugin@marketplace
	Skills       []string          `yaml:"skills"`       // owner/repo/path/to/skill[@ref] on GitHub
}

// Empty reports whether the file asks for nothing for Claude.
func (c Claude) Empty() bool {
	return len(c.Marketplaces) == 0 && len(c.Plugins) == 0 && len(c.Skills) == 0
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
			return "", fmt.Errorf("no %s (or .yml) here or in a parent directory (see: limabean up --help)", FileName)
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
	pluginRE = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)@([A-Za-z0-9][A-Za-z0-9._-]*)$`)
	skillRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)+(@[A-Za-z0-9._/-]+)?$`)
)

// Load reads and checks a project file. Unknown keys are errors, so typos
// don't go unnoticed. The name defaults to the file's directory's.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
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
	return f.Claude.check()
}

func (c Claude) check() error {
	for name, src := range c.Marketplaces {
		if !nameRE.MatchString(name) {
			return fmt.Errorf("claude.marketplaces: %q isn't a marketplace name", name)
		}
		if !githubRE.MatchString(src) && (!gitURLRE.MatchString(src) || strings.Contains(src, "..")) {
			return fmt.Errorf("claude.marketplaces.%s: %q isn't a GitHub owner/repo or an https git URL", name, src)
		}
	}
	for _, p := range c.Plugins {
		m := pluginRE.FindStringSubmatch(p)
		if m == nil {
			return fmt.Errorf("claude.plugins: %q isn't plugin@marketplace", p)
		}
		if _, ok := c.Marketplaces[m[2]]; !ok {
			return fmt.Errorf("claude.plugins: %s comes from marketplace %q; add it under claude.marketplaces", p, m[2])
		}
	}
	seen := map[string]bool{}
	for _, s := range c.Skills {
		if !skillRE.MatchString(s) || strings.Contains(s, "..") {
			return fmt.Errorf("claude.skills: %q isn't owner/repo/path/to/skill (with an optional @ref)", s)
		}
		name := SkillName(s)
		if seen[name] {
			return fmt.Errorf("claude.skills: two skills are named %s", name)
		}
		seen[name] = true
	}
	return nil
}

// SkillName is the directory a skill is installed as: its path's last
// part.
func SkillName(skill string) string {
	path, _, _ := strings.Cut(skill, "@")
	return path[strings.LastIndex(path, "/")+1:]
}

// DefaultName turns a directory's name into a VM name.
func DefaultName(dir string) string {
	n := strings.Trim(invalid.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "-"), "-")
	if n == "" {
		return "project"
	}
	return n
}
