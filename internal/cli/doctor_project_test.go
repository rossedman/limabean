package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorProject(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	file := projectFile(t, "proj", "packs: [cobol]\nnetwork: {allow: [nope]}\nsecrets: [MISSING_KEY]\n")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Dir(file)); err != nil {
		t.Fatal(err)
	}

	// What the schema can't know: packs, presets and secrets on this Mac.
	out, err := h.run("", "doctor")
	for _, want := range []string{"\nProject (", `packs: unknown pack "cobol"`, `network: "nope" isn't a domain or a preset`,
		"secrets not set up on this Mac: MISSING_KEY", "limabean secrets set MISSING_KEY --vm proj", "No VM yet: limabean up", "editor-support"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor is missing %q:\n%s", want, out)
		}
	}
	if err == nil {
		t.Error("a broken project file should fail doctor")
	}

	// A good file, its VM up to date, then out of date.
	rewrite(t, file, "# yaml-language-server: $schema=x\npacks: [go]\ncpus: 2\n")
	h.must("", "up", "--yes")
	out, _ = h.run("", "doctor")
	if !strings.Contains(out, "loads, and its packs, network and secrets are all here") || !strings.Contains(out, "proj (running) matches the file") || strings.Contains(out, "editor-support") {
		t.Errorf("good file:\n%s", out)
	}
	rewrite(t, file, "packs: [go, rust]\ncpus: 2\n")
	if out, _ := h.run("", "doctor"); !strings.Contains(out, "differs from the file in 1 way(s): limabean up --preview") {
		t.Errorf("out of date:\n%s", out)
	}

	// A file that doesn't load, and two files.
	rewrite(t, file, "enviroment: x\n")
	if out, _ := h.run("", "doctor"); !strings.Contains(out, "enviroment") {
		t.Errorf("bad file:\n%s", out)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), ".limabean.yml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _ := h.run("", "doctor"); !strings.Contains(out, "keep one") {
		t.Errorf("two files:\n%s", out)
	}

	// Outside a project, no section.
	chdirTemp(t)
	if out, _ := h.run("", "doctor"); strings.Contains(out, "\nProject (") {
		t.Errorf("outside a project:\n%s", out)
	}
}
