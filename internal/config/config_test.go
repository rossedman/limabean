package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A config written by the bash version of `limabean init`.
const bashConfig = `# limabean config, read by bin/limabean. Applies to VMs created afterwards.

# Host directory with your git config, mounted read-only into VMs.
GIT_CONFIG_DIR="~/.dotfiles/git"

# Where ghq clones repositories inside VMs.
CODE_DIR="~/code"

# VM size; empty uses the template default (4 CPUs, 8 GiB).
CPUS=""
MEMORY=""

# Add each VM to Claude Desktop as an SSH connection (~/.claude/settings.json).
CLAUDE_DESKTOP="yes"
`

func TestLoadBashConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(bashConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(New(), path)
	if err != nil {
		t.Fatal(err)
	}
	if c.GitConfigDir != "~/.dotfiles/git" || c.CodeDir != "~/code" || c.CPUs != 0 || c.Memory != "" || !c.ClaudeDesktop {
		t.Fatalf("got %+v", c)
	}
	if c.AWSDir != "~/.aws" {
		t.Errorf("AWS_DIR default = %q (missing from older configs)", c.AWSDir)
	}
	if c.IDE != "code" {
		t.Errorf("IDE default = %q (missing from older configs)", c.IDE)
	}
}

func TestDefaultsWhenMissing(t *testing.T) {
	c, err := Load(New(), filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if c.GitConfigDir != "~/.config/git" || c.CodeDir != "~/code" || !c.ClaudeDesktop || c.AWSDir != "~/.aws" {
		t.Fatalf("got %+v", c)
	}
}

func TestEnvironmentOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(bashConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIMABEAN_CODE_DIR", "~/src")
	t.Setenv("LIMABEAN_CPUS", "8")
	t.Setenv("LIMABEAN_CLAUDE_DESKTOP", "no")
	c, err := Load(New(), path)
	if err != nil {
		t.Fatal(err)
	}
	if c.CodeDir != "~/src" || c.CPUs != 8 || c.ClaudeDesktop {
		t.Fatalf("got %+v", c)
	}
}

func TestBadCPUs(t *testing.T) {
	t.Setenv("LIMABEAN_CPUS", "lots")
	if _, err := Load(New(), filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "CPUS") {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config")
	want := &Config{Path: path, GitConfigDir: "~/dots/git", CodeDir: "~/work", CPUs: 6, Memory: "12", ClaudeDesktop: false, AWSDir: "", IDE: "cursor"}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(New(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
}

func TestExpand(t *testing.T) {
	home, _ := os.UserHomeDir()
	if Expand("~/x") != filepath.Join(home, "x") || Expand("~") != home || Expand("/abs") != "/abs" {
		t.Fatal("Expand mismatch")
	}
}

func TestEmptyEnvironmentVariableIsAValue(t *testing.T) {
	t.Setenv("LIMABEAN_AWS_DIR", "")
	c, err := Load(New(), filepath.Join(t.TempDir(), "none"))
	if err != nil {
		t.Fatal(err)
	}
	if c.AWSDir != "" {
		t.Fatalf("LIMABEAN_AWS_DIR=\"\" should turn the AWS mount off, got %q", c.AWSDir)
	}
}
