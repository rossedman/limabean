package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommand(t *testing.T) {
	ctx := context.Background()
	if out, err := RunCommand(ctx, "sh", "-c", "printf hi"); err != nil || string(out) != "hi" {
		t.Fatalf("RunCommand = %q, %v", out, err)
	}
	if _, err := RunCommand(ctx, "sh", "-c", "echo nope >&2; exit 2"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("stderr not in error: %v", err)
	}
	if _, err := RunCommand(ctx, "sh", "-c", "exit 2"); err == nil {
		t.Fatal("silent failure should still fail")
	}
}

func TestResolveFailures(t *testing.T) {
	r := Resolver{
		HasProgram: func(string) bool { return true },
		Run:        func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("x") },
	}
	ctx := context.Background()
	if _, err := r.Resolve(ctx, "op://v/i/f"); err == nil || !strings.Contains(err.Error(), "op couldn't read") {
		t.Errorf("op failure: %v", err)
	}
	if _, err := r.Resolve(ctx, "cmd:false"); err == nil || !strings.Contains(err.Error(), "command failed") {
		t.Errorf("cmd failure: %v", err)
	}
}

func TestFileErrors(t *testing.T) {
	dir := t.TempDir()
	if _, errs := LoadFile(dir); len(errs) == 0 { // a directory
		t.Error("loading a directory should report an error")
	}
	if list, errs := LoadFile(filepath.Join(dir, "missing.env")); list != nil || errs != nil {
		t.Error("a missing file is empty, not an error")
	}
	if err := Put(dir, "A", "x"); err == nil { // the path is a directory
		t.Error("Put onto a directory should fail")
	}
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	if err := Put(filepath.Join(ro, "sub", "secrets.env"), "A", "x"); err == nil {
		t.Error("Put under a read-only directory should fail")
	}
	if err := Put(filepath.Join(ro, "secrets.env"), "A", "x"); err == nil {
		t.Error("Put into a read-only directory should fail")
	}
}

func TestParseVeryLongLine(t *testing.T) {
	_, errs := Parse(strings.NewReader("A="+strings.Repeat("x", 2<<20)+"\n"), "big.env")
	if len(errs) == 0 {
		t.Fatal("a line over the scanner limit should be reported")
	}
}
