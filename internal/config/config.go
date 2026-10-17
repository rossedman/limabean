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
	KeyClaudeDesktop = "claude_desktop"
	KeyAWSDir        = "aws_dir"
	KeyPlain         = "plain"
	KeyTemplates     = "templates"
)

// Config is the resolved settings.
type Config struct {
	Path          string // the config file
	GitConfigDir  string // host directory mounted read-only at /mnt/host/git
	CodeDir       string // ghq root inside VMs
	CPUs          int    // 0: template default
	Memory        string // GiB; "": template default
	ClaudeDesktop bool   // register VMs as SSH connections in Claude Desktop
	AWSDir        string // mounted read-only at ~/.aws in VMs; "": never
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
	v.SetDefault(KeyClaudeDesktop, "yes")
	v.SetDefault(KeyAWSDir, "~/.aws")
	v.SetEnvPrefix("LIMABEAN")
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
	return &Config{
		Path:          path,
		GitConfigDir:  v.GetString(KeyGitConfigDir),
		CodeDir:       v.GetString(KeyCodeDir),
		CPUs:          cpus,
		Memory:        strings.TrimSpace(v.GetString(KeyMemory)),
		ClaudeDesktop: yes(v.GetString(KeyClaudeDesktop)),
		AWSDir:        v.GetString(KeyAWSDir),
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
	claude := "no"
	if c.ClaudeDesktop {
		claude = "yes"
	}
	body := fmt.Sprintf(`# limabean config. Applies to VMs created afterwards.

# Host directory with your git config, mounted read-only into VMs.
GIT_CONFIG_DIR=%q

# Where ghq clones repositories inside VMs.
CODE_DIR=%q

# VM size; empty uses the template default (4 CPUs, 8 GiB).
CPUS=%q
MEMORY=%q

# Add each VM to Claude Desktop as an SSH connection (~/.claude/settings.json).
CLAUDE_DESKTOP=%q

# AWS CLI config, mounted read-only at ~/.aws in VMs when it exists. Empty: never.
AWS_DIR=%q
`, c.GitConfigDir, c.CodeDir, cpus, c.Memory, claude, c.AWSDir)
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
