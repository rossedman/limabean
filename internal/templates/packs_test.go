package templates

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The packs in the repository: each parses, has a description, pins real
// versions, and core has the tools limabean relies on.
func TestRepositoryPacks(t *testing.T) {
	packs, err := Packs(filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Pack{}
	for _, p := range packs {
		byName[p.Name] = p
		if p.Description == "" || strings.HasPrefix(p.Description, "[") {
			t.Errorf("%s: the first line should be a # description, got %q", p.Name, p.Description)
		}
		if len(p.Tools) == 0 && len(p.Apt) == 0 {
			t.Errorf("%s installs nothing", p.Name)
		}
	}
	for _, want := range []string{"core", "go", "python", "node", "kubernetes", "infra", "containers", "db", "api", "supply-chain", "rust", "java", "ruby", "claude"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("missing pack %s", want)
		}
	}
	for _, tool := range []string{"ghq", "gh", "bat", "fzf", "direnv"} {
		if !slices.Contains(byName["core"].Tools, tool) {
			t.Errorf("core must have %s: limabean and the shell use it", tool)
		}
	}
	if !slices.Contains(byName["db"].Apt, "postgresql-client") || byName["db"].AptList() == "" {
		t.Errorf("db: %+v", byName["db"])
	}
	if byName["go"].AptList() != "" {
		t.Error("a pack without apt packages has no apt list")
	}
	if c := byName["containers"].Contents(); !strings.Contains(c, "act") || !strings.Contains(c, "skopeo") {
		t.Errorf("containers contents = %q", c)
	}
}

func TestPacksSkipsOddFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "packs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"ok.toml":       "# OK pack\n[tools]\njq = \"1.7\"\n",
		"ok.apt":        "# comment\nfoo bar\n\nbaz # trailing\n",
		"Bad_Name.toml": "# skipped\n",
		"notes.txt":     "ignored",
		"nodesc.toml":   "[tools]\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packs, err := Packs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 2 || packs[1].Name != "ok" || strings.Join(packs[1].Apt, ",") != "foo,bar,baz" || packs[1].Tools[0] != "jq" {
		t.Fatalf("packs = %+v", packs)
	}
	if packs, err := Packs(filepath.Join(root, "none")); err != nil || len(packs) != 0 {
		t.Errorf("no packs dir: %v %v", packs, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unreadable.toml"), nil, 0o000); err == nil && os.Getuid() != 0 {
		if _, err := Packs(root); err == nil {
			t.Error("an unreadable pack should fail")
		}
	}
}

func TestResolvePacks(t *testing.T) {
	all := []Pack{{Name: "core"}, {Name: "go"}, {Name: "node"}, {Name: "rust"}}
	got, err := ResolvePacks(all, []string{"node", "go", "core"}, []string{"rust", "go"}, []string{"node"})
	if err != nil || strings.Join(got, ",") != "go,rust" {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := ResolvePacks(all, nil, []string{"cobol"}, nil); err == nil || !strings.Contains(err.Error(), "available: go, node, rust") {
		t.Errorf("unknown pack: %v", err)
	}
	if _, err := ResolvePacks(all, nil, nil, []string{"core"}); err == nil || !strings.Contains(err.Error(), "every VM") {
		t.Errorf("removing core: %v", err)
	}
	if got, _ := ResolvePacks(all, nil, nil, nil); len(got) != 0 {
		t.Errorf("nothing: %v", got)
	}
	if got := SplitList(" Go, node rust,,"); strings.Join(got, "|") != "go|node|rust" {
		t.Errorf("SplitList = %v", got)
	}
}

// User packs from ~/.config/limabean/templates/packs are copied in, win over
// built-in ones, and disappear when removed there.
func TestUserPacks(t *testing.T) {
	user := t.TempDir()
	cache := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(user, "packs", "zig.toml"), "# Zig\n[tools]\nzig = \"0.15.2\"\n")
	write(filepath.Join(user, "packs", "zig.apt"), "inotify-tools\n")
	write(filepath.Join(user, "packs", "go.toml"), "# My Go\n[tools]\ngo = \"1.26.0\"\n")
	write(filepath.Join(user, "packs", "README.md"), "ignored")
	d := Dirs{User: user, Cache: cache}
	fsys := os.DirFS(filepath.Join("..", ".."))
	root, err := d.Root(fsys)
	if err != nil {
		t.Fatal(err)
	}
	packs, err := Packs(root)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Pack{}
	for _, p := range packs {
		byName[p.Name] = p
	}
	if byName["zig"].Description != "Zig" || strings.Join(byName["zig"].Apt, ",") != "inotify-tools" {
		t.Errorf("user pack: %+v", byName["zig"])
	}
	if byName["go"].Description != "My Go" || byName["rust"].Name == "" {
		t.Errorf("a user pack should replace the built-in one, and leave the others: go=%+v", byName["go"])
	}
	if strings.Count(strings.Join(names(packs), ","), "go") != 1 {
		t.Errorf("go listed twice: %v", names(packs))
	}

	os.Remove(filepath.Join(user, "packs", "zig.toml"))
	if root, err = d.Root(fsys); err != nil {
		t.Fatal(err)
	}
	packs, _ = Packs(root)
	for _, p := range packs {
		if p.Name == "zig" {
			t.Error("a removed user pack should be gone")
		}
	}
}

func names(packs []Pack) []string {
	var out []string
	for _, p := range packs {
		out = append(out, p.Name)
	}
	return out
}
