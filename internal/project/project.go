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

// FileName is the project file's name.
const FileName = ".limabean.yaml"

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
}

// Find looks for the project file in dir and its parents, and returns its
// path.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, FileName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s here or in a parent directory (see: limabean up --help)", FileName)
		}
		dir = parent
	}
}

var (
	nameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)
	secretRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	invalid  = regexp.MustCompile(`[^a-z0-9-]+`)
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
