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
	KeyPlain         = "plain"
	KeyTemplates     = "templates"
)

// DefaultPacks are the packs new VMs get unless PACKS says otherwise.
const DefaultPacks = "go,python,node,kubernetes,infra,containers,db"

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
	}, nil
}

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

# Packs of tools new VMs get besides core (limabean packs lists them);
# new --with/--without adjust it per VM.
PACKS=%q

# Create VMs in seconds by cloning a provisioned image, built in the
# background and kept fresh (limabean images). no: provision every VM.
IMAGES=%q
`, c.GitConfigDir, c.CodeDir, codeDisk, cpus, c.Memory, disk, yesNo(c.ClaudeDesktop), c.AWSDir, c.IDE, c.Packs, yesNo(c.Images))
	return os.WriteFile(c.Path, []byte(body), 0o600)
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
