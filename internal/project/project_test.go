package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindAndLoad(t *testing.T) {
	root := filepath.Join(t.TempDir(), "My App")
	write(t, filepath.Join(root, FileName), `
packs: [rust, claude]
without: [kubernetes]
cpus: 6
memory: 12
disk: 30
network:
  allow: [github]
  locked: true
secrets: [OPENAI_API_KEY]
ports: [3000]
setup: |
  make deps
`)
	sub := filepath.Join(root, "cmd", "app")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := Find(sub)
	if err != nil || path != filepath.Join(root, FileName) {
		t.Fatalf("Find = %q, %v", path, err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "my-app" || f.CPUs != 6 || f.Disk != 30 || !f.Network.Locked || f.Packs[1] != "claude" || f.Setup != "make deps\n" || f.Ports[0] != 3000 {
		t.Errorf("Load = %+v", f)
	}
	if _, err := Find(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no .limabean.yaml") {
		t.Errorf("Find with none: %v", err)
	}
}

func TestLoadRejects(t *testing.T) {
	dir := t.TempDir()
	for body, want := range map[string]string{
		"pakcs: [rust]":            "pakcs",
		"name: bad_name":           "name",
		"env: ../x":                "env",
		"packs: [--with]":          "pack",
		"secrets: [X=cmd:id]":      "only names",
		"cpus: -1":                 "positive",
		"ports: [70000]":           "out of range",
		"repo: --upload-pack=evil": "isn't a repository",
		"network: {allow: github}": "cannot unmarshal",
	} {
		p := filepath.Join(dir, FileName)
		write(t, p, body)
		if _, err := Load(p); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v", body, err)
		}
	}
	p := filepath.Join(dir, "claude", FileName)
	write(t, p, `claude:
  marketplaces:
    - marketplace: rossedman/guardrails
      plugins: [guardrails@guardrails]
    - marketplace: https://git.example.com/team/plugins.git
      plugins: [review@team, deploy@team]
`)
	f, err := Load(p)
	if err != nil || f.Claude.Empty() || f.Claude.Marketplaces[1].Name() != "team" || len(f.Claude.Plugins()) != 3 {
		t.Fatalf("claude: %+v %v", f, err)
	}
	if (Marketplace{}).Name() != "" || !(Claude{}).Empty() {
		t.Error("Name or Empty")
	}
	p = filepath.Join(dir, "empty", FileName)
	write(t, p, "")
	if f, err := Load(p); err != nil || f.Name != "empty" {
		t.Errorf("an empty file: %+v %v", f, err)
	}
	if _, err := Load(filepath.Join(dir, "none", FileName)); err == nil {
		t.Error("a missing file should fail")
	}
	if DefaultName("/x/___") != "project" || DefaultName("/x/Foo.Bar") != "foo-bar" {
		t.Errorf("DefaultName: %q %q", DefaultName("/x/___"), DefaultName("/x/Foo.Bar"))
	}
}

func TestFindYml(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".limabean.yml"), "cpus: 2\n")
	if p, err := Find(dir); err != nil || filepath.Base(p) != ".limabean.yml" {
		t.Fatalf("Find = %q, %v", p, err)
	}
	write(t, filepath.Join(dir, FileName), "cpus: 2\n")
	if _, err := Find(dir); err == nil || !strings.Contains(err.Error(), "keep one") {
		t.Errorf("both: %v", err)
	}
}
