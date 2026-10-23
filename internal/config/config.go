// Package config loads limabean's settings with Viper.
//
// Settings come from, highest priority first: command-line flags bound by
// the commands, LIMABEAN_* environment variables, and the config file
// (~/.config/limabean/config). The file is a KEY="value" shell file, read
// with Viper's dotenv format, so files written by the bash version of
// `limabean init` keep working.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"github.com/rossedman/limabean/internal/atomic"
)

// Keys, as written in the config file. Viper lower-cases them.
const (
	KeyGitConfigDir  = "git_config_dir"
	KeyCodeDir       = "code_dir"
	KeyCPUs          = "cpus"
	KeyMemory        = "memory"
	KeyDisk          = "disk"
	KeyClaudeDesktop = "claude_desktop"
	KeyAWSDir        = "aws_dir"
	KeyIDE           = "ide"
	KeyCodeDisk      = "code_disk"
	KeyPacks         = "packs"
	KeyImages        = "images"
	KeyCACerts       = "ca_certs"
	KeyUpdateCheck   = "update_check"
	KeyPlain         = "plain"
	KeyTemplates     = "templates"
)

// DefaultPacks are the packs new VMs get unless PACKS says otherwise: none
// besides core, so a VM, a project file or a recipe names what it needs.
const DefaultPacks = ""

// Config is the resolved settings.
type Config struct {
	Path          string // the config file
	GitConfigDir  string // host directory mounted read-only at /mnt/host/git
	CodeDir       string // ghq root inside VMs
	CPUs          int    // 0: template default
	Memory        string // GiB; "": template default
	Disk          int    // GiB of the system disk; 0: template default
	ClaudeDesktop bool   // register VMs as SSH connections in Claude Desktop
	AWSDir        string // mounted read-only at ~/.aws in VMs; "": never
	IDE           string // editor for `limabean code`, e.g. code-insiders; "": the first installed
	CodeDisk      int    // GiB of the disk holding CodeDir in new VMs; 0: none
	Packs         string // packs new VMs get besides core, comma-separated
	Images        bool   // create VMs by cloning a cached, provisioned image
	CACerts       bool   // VMs trust the root CAs an admin added to the Mac
	UpdateCheck   bool   // say when a newer limabean is released
}

// DefaultPath is ~/.config/limabean/config.
func DefaultPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "limabean", "config")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "limabean", "config")
}

// New returns a Viper set up with limabean's defaults and environment.
func New() *viper.Viper {
	v := viper.New()
	v.SetDefault(KeyGitConfigDir, "~/.config/git")
	v.SetDefault(KeyCodeDir, "~/code")
	v.SetDefault(KeyCPUs, 0)
	v.SetDefault(KeyMemory, "")
	v.SetDefault(KeyDisk, "")
	v.SetDefault(KeyClaudeDesktop, "yes")
	v.SetDefault(KeyAWSDir, "~/.aws")
	v.SetDefault(KeyIDE, "")
	v.SetDefault(KeyCodeDisk, "50")
	v.SetDefault(KeyPacks, DefaultPacks)
	v.SetDefault(KeyImages, "yes")
	v.SetDefault(KeyCACerts, "yes")
	v.SetDefault(KeyUpdateCheck, "yes")
	v.SetEnvPrefix("LIMABEAN")
	// An empty variable is a value: LIMABEAN_AWS_DIR="" turns the mount off.
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()
	return v
}

// Load reads the config file at path into v (a missing file is fine) and
// returns the resolved settings.
func Load(v *viper.Viper, path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err == nil {
		v.SetConfigFile(path)
		v.SetConfigType("dotenv")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	cpus := 0
	if s := strings.TrimSpace(v.GetString(KeyCPUs)); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("CPUS must be a whole number, got %q", s)
		}
		cpus = n
	}
	disk := 0
	if s := strings.TrimSpace(v.GetString(KeyDisk)); s != "" {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "GiB"))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("DISK must be a size in GiB, like 20, or empty, got %q", s)
		}
		disk = n
	}
	codeDisk := 0
	if s := strings.TrimSpace(v.GetString(KeyCodeDisk)); s != "" {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "GiB"))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("CODE_DISK must be a size in GiB, like 50, or empty, got %q", s)
		}
		codeDisk = n
	}
	return &Config{
		Path:          path,
		GitConfigDir:  v.GetString(KeyGitConfigDir),
		CodeDir:       v.GetString(KeyCodeDir),
		CPUs:          cpus,
		Memory:        strings.TrimSpace(v.GetString(KeyMemory)),
		Disk:          disk,
		ClaudeDesktop: yes(v.GetString(KeyClaudeDesktop)),
		AWSDir:        v.GetString(KeyAWSDir),
		IDE:           strings.TrimSpace(v.GetString(KeyIDE)),
		CodeDisk:      codeDisk,
		Packs:         strings.TrimSpace(v.GetString(KeyPacks)),
		Images:        yes(v.GetString(KeyImages)),
		CACerts:       yes(v.GetString(KeyCACerts)),
		UpdateCheck:   yes(v.GetString(KeyUpdateCheck)),
	}, nil
}

// yes reports whether a setting's value reads as true: yes, y, true, 1 or
// on, in any case.
func yes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "true", "1", "on":
		return true
	}
	return false
}

// Save writes the settings in the commented KEY="value" format.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o700); err != nil {
		return err
	}
	cpus := ""
	if c.CPUs > 0 {
		cpus = strconv.Itoa(c.CPUs)
	}
	disk := ""
	if c.Disk > 0 {
		disk = strconv.Itoa(c.Disk)
	}
	codeDisk := ""
	if c.CodeDisk > 0 {
		codeDisk = strconv.Itoa(c.CodeDisk)
	}
	yesNo := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	body := fmt.Sprintf(`# limabean config. Applies to VMs created afterwards.

# Host directory with your git config, mounted read-only into VMs.
GIT_CONFIG_DIR=%q

# Where ghq clones repositories inside VMs.
CODE_DIR=%q

# GiB of the disk that holds CODE_DIR in new VMs, so repositories survive
# limabean rebuild. Empty: CODE_DIR is on the VM's own disk.
CODE_DISK=%q

# VM size: CPUs, and GiB of memory and of system disk. Empty uses the
# template default (2 CPUs, 4 GiB, 10 GiB).
CPUS=%q
MEMORY=%q
DISK=%q

# Add each VM to Claude Desktop as an SSH connection (~/.claude/settings.json).
CLAUDE_DESKTOP=%q

# AWS CLI config, mounted read-only at ~/.aws in VMs when it exists. Empty: never.
AWS_DIR=%q

# Editor that limabean code opens VMs in: code (VS Code), code-insiders,
# cursor, windsurf, codium, or another editor with VS Code's --remote flag.
# Empty: the first of those that's installed.
IDE=%q

# Packs of tools new VMs get besides core (limabean packs lists them).
# Empty: only core; new asks in a terminal, --with/--without adjust it, and
# a project's .limabean.yaml lists its own.
PACKS=%q

# Create VMs in seconds by cloning a provisioned image, built in the
# background and kept fresh (limabean images). no: provision every VM.
IMAGES=%q

# VMs trust the root certificates an admin added to the Mac, such as a
# company proxy's (Zscaler and the like), so HTTPS through it works.
# no: VMs trust only Ubuntu's own list.
CA_CERTS=%q

# Once a day, check GitHub for a newer limabean and say so in a terminal.
# no: never check.
UPDATE_CHECK=%q
`, c.GitConfigDir, c.CodeDir, codeDisk, cpus, c.Memory, disk, yesNo(c.ClaudeDesktop), c.AWSDir, c.IDE, c.Packs, yesNo(c.Images), yesNo(c.CACerts), yesNo(c.UpdateCheck))
	return atomic.WriteFile(c.Path, []byte(body), 0o600)
}

// Expand replaces a leading ~ with the home directory. Config values keep
// the ~ so Lima can expand it itself.
func Expand(path string) string {
	home, _ := os.UserHomeDir()
	switch {
	case path == "~":
		return home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(home, path[2:])
	}
	return path
}

// Dir is the directory the config file lives in; secrets live there too.
func (c *Config) Dir() string { return filepath.Dir(c.Path) }

// Keys are the settings in the config file's order, then the ones only set
// by a flag or the environment.
var Keys = []string{
	KeyGitConfigDir, KeyCodeDir, KeyCodeDisk, KeyCPUs, KeyMemory, KeyDisk, KeyClaudeDesktop,
	KeyAWSDir, KeyIDE, KeyPacks, KeyImages, KeyCACerts, KeyUpdateCheck, KeyPlain, KeyTemplates,
}

// Source says where key's value comes from, short of a flag: "env" for a
// LIMABEAN_* variable, "file" for the config file, or "default".
func Source(v *viper.Viper, key string) string {
	if _, ok := os.LookupEnv("LIMABEAN_" + strings.ToUpper(key)); ok {
		return "env"
	}
	if v.InConfig(key) {
		return "file"
	}
	return "default"
}
