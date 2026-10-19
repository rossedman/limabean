package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The idle check's pattern for Claude Code, run with pgrep as in a VM,
// against only this test's own processes.
func TestIdleScriptSeesClaudeCode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pgrep -f reads /proc, as in a VM")
	}
	if _, err := exec.LookPath("pgrep"); err != nil {
		t.Skip("needs pgrep")
	}
	if !strings.Contains(idleScript, "'"+claudeCodeRE+"'") {
		t.Fatal("idleScript doesn't use claudeCodeRE")
	}
	dir := t.TempDir()
	start := func(path string, args ...string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(path, args...)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		time.Sleep(100 * time.Millisecond)
	}
	running := func() bool {
		return exec.Command("pgrep", "-P", strconv.Itoa(os.Getpid()), "-f", claudeCodeRE).Run() == nil
	}

	// Paths with claude in them, like Claude Desktop's ~/.claude/remote, or
	// a project called claude-tools, aren't Claude Code.
	start(filepath.Join(dir, ".claude", "remote", "server"), "--socket", "x")
	start(filepath.Join(dir, "claude-tools", "build"))
	if running() {
		t.Error("a program with claude only in its path counted as Claude Code")
	}
	// A headless run, by its program's name.
	start(filepath.Join(dir, "bin", "claude"), "-p", "fix the tests")
	if !running() {
		t.Error("claude -p didn't count as Claude Code")
	}
}
