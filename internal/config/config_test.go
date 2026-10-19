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
	if c.IDE != "" {
		t.Errorf("IDE default = %q, want empty: the first editor installed", c.IDE)
	}
	if c.CodeDisk != 50 {
		t.Errorf("CODE_DISK default = %d (missing from older configs)", c.CodeDisk)
	}
	if c.Packs != DefaultPacks {
		t.Errorf("PACKS default = %q (missing from older configs)", c.Packs)
	}
	if !c.Images {
		t.Error("IMAGES should default to yes (missing from older configs)")
	}
	if !c.CACerts {
		t.Error("CA_CERTS should default to yes (missing from older configs)")
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
	want := &Config{Path: path, GitConfigDir: "~/dots/git", CodeDir: "~/work", CPUs: 6, Memory: "12", Disk: 20, ClaudeDesktop: false, AWSDir: "", IDE: "cursor", CodeDisk: 0, Packs: "go,rust", Images: false}
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

func TestCodeDisk(t *testing.T) {
	for in, want := range map[string]int{"": 0, "0": 0, "80": 80, "120GiB": 120} {
		t.Setenv("LIMABEAN_CODE_DISK", in)
		c, err := Load(New(), filepath.Join(t.TempDir(), "x"))
		if err != nil || c.CodeDisk != want {
			t.Errorf("CODE_DISK=%q: %d, %v", in, c.CodeDisk, err)
		}
	}
	for _, bad := range []string{"big", "-5"} {
		t.Setenv("LIMABEAN_CODE_DISK", bad)
		if _, err := Load(New(), filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "CODE_DISK") {
			t.Errorf("CODE_DISK=%q: %v", bad, err)
		}
	}
	path := filepath.Join(t.TempDir(), "config")
	if err := (&Config{Path: path, CodeDisk: 75}).Save(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIMABEAN_CODE_DISK", "")
	os.Unsetenv("LIMABEAN_CODE_DISK")
	if c, err := Load(New(), path); err != nil || c.CodeDisk != 75 {
		t.Errorf("saved CODE_DISK: %v, %v", c, err)
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

func TestDisk(t *testing.T) {
	for in, want := range map[string]int{"": 0, "20": 20, "40GiB": 40} {
		t.Setenv("LIMABEAN_DISK", in)
		c, err := Load(New(), filepath.Join(t.TempDir(), "x"))
		if err != nil || c.Disk != want {
			t.Errorf("DISK=%q: %d, %v", in, c.Disk, err)
		}
	}
	t.Setenv("LIMABEAN_DISK", "big")
	if _, err := Load(New(), filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "DISK") {
		t.Errorf("bad DISK: %v", err)
	}
}
