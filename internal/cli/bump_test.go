package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const bumpTOML = `# pins
[tools]
ripgrep = "15.2.0"
helm = "4.3.1"   # charts
bat = "0.26.1"
go = "1.27.2"
gone = "1.0.0"
`

func TestBump(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "base.toml")
	write := func() {
		if err := os.WriteFile(file, []byte(bumpTOML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write()
	for spec, v := range map[string]string{
		"ripgrep": "15.3.0",                   // a minor in series
		"helm":    "5.0.1", "helm@4": "4.4.2", // a new major
		"bat": "0.27.0", "bat@0.26": "0.26.1", // 0.x: a new minor is a new series
		"go": "1.27.2", // up to date
	} {
		h.world.latest[spec] = v
	}

	if _, err := h.run("", "bump", file); err == nil || !strings.Contains(err.Error(), "mise isn't installed") {
		t.Fatalf("without mise on PATH: %v", err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "mise"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// A dry run shows everything and writes nothing.
	out, err := h.run("", "bump", file, "--dry-run")
	if !errors.Is(err, errSilent) {
		t.Fatalf("a failed lookup should fail the run: %v", err)
	}
	for _, want := range []string{"ripgrep", "15.3.0", "4.4.2", "5.0.1 available (--major)", "0.27.0 available (--major)", "gone", "lookup failed: mise ERROR no such tool", "2 pin(s) would change"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run missing %q:\n%s", want, out)
		}
	}
	if b, _ := os.ReadFile(file); string(b) != bumpTOML {
		t.Fatal("--dry-run wrote the file")
	}

	// A real run rewrites only the version strings.
	h.run("", "bump", file)
	b, _ := os.ReadFile(file)
	want := strings.NewReplacer(`ripgrep = "15.2.0"`, `ripgrep = "15.3.0"`, `helm = "4.3.1"`, `helm = "4.4.2"`).Replace(bumpTOML)
	if string(b) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", b, want)
	}

	// --major takes the newest of everything.
	write()
	h.run("", "bump", file, "--major")
	b, _ = os.ReadFile(file)
	for _, want := range []string{`helm = "5.0.1"   # charts`, `bat = "0.27.0"`, `go = "1.27.2"`, `gone = "1.0.0"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("--major: missing %q in\n%s", want, b)
		}
	}

	// Up to date, and a file with no pins.
	delete(h.world.latest, "helm")
	h.world.latest["gone"] = "1.0.0"
	h.world.latest["helm"] = "5.0.1"
	if out := h.must("", "bump", file); !strings.Contains(out, "all 5 pins are up to date") {
		t.Errorf("up to date:\n%s", out)
	}
	empty := filepath.Join(dir, "empty.toml")
	os.WriteFile(empty, []byte("[env]\nX = \"1\"\n"), 0o644)
	if out := h.must("", "bump", empty); !strings.Contains(out, "no pinned tools") {
		t.Errorf("no pins:\n%s", out)
	}
	if _, err := h.run("", "bump", filepath.Join(dir, "missing.toml")); err == nil {
		t.Error("a missing file should fail")
	}

	// Without files: templates/mise/*.toml in the current directory.
	t.Chdir(dir)
	if _, err := h.run("", "bump"); err == nil || !strings.Contains(err.Error(), "no templates/mise") {
		t.Errorf("no templates here: %v", err)
	}
	os.MkdirAll(filepath.Join(dir, "templates", "mise"), 0o755)
	write()
	os.Rename(file, filepath.Join(dir, "templates", "mise", "base.toml"))
	if out := h.must("", "bump"); !strings.Contains(out, "templates/mise/base.toml: updated 2 pin(s)") {
		t.Errorf("default files:\n%s", out)
	}
}

func TestBumpInVM(t *testing.T) {
	h := newHarness(t)
	file := filepath.Join(t.TempDir(), "base.toml")
	os.WriteFile(file, []byte("[tools]\nripgrep = \"15.2.0\"\n"), 0o644)
	h.world.latest["ripgrep"] = "15.3.0"
	if _, err := h.run("", "bump", file, "--vm", "nope"); err == nil {
		t.Error("a missing VM should fail")
	}
	h.world.vms["dev"] = "Stopped"
	if _, err := h.run("", "bump", file, "--vm", "dev"); err == nil || !strings.Contains(err.Error(), "dev is stopped") {
		t.Errorf("a stopped VM: %v", err)
	}
	h.world.vms["dev"] = "Running"
	h.must("", "bump", file, "--vm", "dev")
	if b, _ := os.ReadFile(file); !strings.Contains(string(b), `"15.3.0"`) {
		t.Fatalf("bump in the VM didn't update:\n%s", b)
	}
	if h.world.called("mise latest") || !strings.Contains(strings.Join(h.world.calls, "\n"), "lima-dev mise latest 'ripgrep'") {
		t.Errorf("should ask the VM, not the Mac: %v", h.world.calls)
	}
}

func TestMiseLatestAndFirstLine(t *testing.T) {
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "mise"), []byte("#!/bin/sh\n[ \"$2\" = ok ] && { echo 1.2.3; exit 0; }\necho 'mise ERROR nope' >&2; exit 1\n"), 0o755)
	t.Setenv("PATH", bin)
	if v, err := miseLatest(t.Context(), "ok"); err != nil || v != "1.2.3" {
		t.Errorf("miseLatest = %q, %v", v, err)
	}
	_, err := miseLatest(t.Context(), "bad")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || firstLine(err) != "mise ERROR nope" {
		t.Errorf("firstLine of mise's error = %q", firstLine(err))
	}
}
