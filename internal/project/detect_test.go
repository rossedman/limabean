package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// files writes a repository: path → contents.
func files(t *testing.T, tree map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range tree {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// packOf is the built-in packs' tools, as far as these tests need.
func packOf(tool string) string {
	switch tool {
	case "node", "pnpm":
		return "node"
	case "go":
		return "go"
	case "terraform":
		return "infra"
	case "ripgrep":
		return "core"
	}
	return ""
}

func detect(t *testing.T, dir string) (*Draft, *File) {
	t.Helper()
	d, err := Detect(dir, packOf)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse([]byte(d.Render()), filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("the draft doesn't load: %v\n%s", err, d.Render())
	}
	return d, f
}

func TestDetectMonorepo(t *testing.T) {
	dir := files(t, map[string]string{
		"api/go.mod":                  "module x\n",
		"web/package.json":            `{"devDependencies": {"vite": "7"}}`,
		"web/pnpm-lock.yaml":          "",
		"compose.yaml":                "services: {}\n",
		".tool-versions":              "nodejs 22.1.0\njust 1.40.0\nzig system\nripgrep 15.0.0\n",
		".env.example":                "OPENAI_API_KEY=\nPORT=3000\nexport DB_PASSWORD=x\n",
		"node_modules/x/package.json": "{}",
	})
	d, f := detect(t, dir)
	if !slices.Equal(f.Packs, []string{"go", "node"}) || f.Tools["just"] != "1.40.0" || len(f.Tools) != 1 {
		t.Errorf("packs %v, tools %v", f.Packs, f.Tools)
	}
	if !slices.Equal(f.Ports, []int{5173}) {
		t.Errorf("ports %v", f.Ports)
	}
	if f.Setup != "(cd api && go mod download)\n(cd web && pnpm install --frozen-lockfile)\n" || f.Start != "docker compose up -d\n" {
		t.Errorf("setup %q, start %q", f.Setup, f.Start)
	}
	out := d.Render()
	for _, want := range []string{SchemaComment, "# secrets: [DB_PASSWORD, OPENAI_API_KEY]", "packs: [go, node]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	found := strings.Join(d.Found(), "\n")
	for _, want := range []string{"go: api/go.mod", "node: .tool-versions, web/package.json", ".tool-versions pins node 22.1.0; the node pack has its own version", "start: compose.yaml", "tools: just from .tool-versions", "pins ripgrep 15.0.0; every VM has it (core)"} {
		if !strings.Contains(found, want) {
			t.Errorf("found is missing %q:\n%s", want, found)
		}
	}
}

func TestDetectEachKind(t *testing.T) {
	for _, c := range []struct {
		tree  map[string]string
		packs []string
		setup string
		ports []int
	}{
		{map[string]string{"package.json": "{}", "package-lock.json": ""}, []string{"node"}, "npm ci\n", nil},
		{map[string]string{"package.json": `{"next": "15"}`, "yarn.lock": ""}, []string{"node"}, "npx --yes yarn install\n", []int{3000}},
		{map[string]string{"package.json": "{}", "bun.lock": ""}, []string{"node"}, "bun install\n", nil},
		{map[string]string{"pyproject.toml": "dependencies = ['fastapi']", "uv.lock": ""}, []string{"python"}, "uv sync\n", []int{8000}},
		{map[string]string{"pyproject.toml": "", "poetry.lock": "", "manage.py": ""}, []string{"python"}, "uvx poetry install\n", []int{8000}},
		{map[string]string{"requirements.txt": "flask\n"}, []string{"python"}, "uv venv && uv pip install -r requirements.txt\n", []int{5000}},
		{map[string]string{"Cargo.toml": ""}, []string{"rust"}, "cargo fetch\n", nil},
		{map[string]string{"Gemfile": "gem 'rails'"}, []string{"ruby"}, "bundle install\n", []int{3000}},
		{map[string]string{"mix.exs": "{:phoenix, \"~> 1.8\"}"}, []string{"elixir"}, "mix local.hex --force && mix deps.get\n", []int{4000}},
		{map[string]string{"pom.xml": "spring-boot", "mvnw": ""}, []string{"java"}, "./mvnw -q dependency:go-offline\n", []int{8080}},
		{map[string]string{"build.gradle.kts": ""}, []string{"java"}, "gradle --quiet dependencies\n", nil},
		{map[string]string{"app.csproj": ""}, []string{"dotnet"}, "dotnet restore\n", nil},
		{map[string]string{"main.tf": "", "Chart.yaml": ""}, []string{"infra", "kubernetes"}, "", nil},
	} {
		_, f := detect(t, files(t, c.tree))
		if !slices.Equal(f.Packs, c.packs) || f.Setup != c.setup || !slices.Equal(f.Ports, c.ports) {
			t.Errorf("%v: packs %v, setup %q, ports %v", c.tree, f.Packs, f.Setup, f.Ports)
		}
	}
}

// Nothing to go on: a file that says so and still loads.
func TestDetectNothing(t *testing.T) {
	d, f := detect(t, files(t, map[string]string{"README.md": "hi"}))
	if !d.Empty() || len(f.Packs) != 0 || !strings.Contains(d.Render(), "limabean recipes") {
		t.Errorf("empty:\n%s", d.Render())
	}
}

// Detect reads the repository only: a symlink out of it is ignored, a
// directory whose name would change setup's meaning isn't looked into, and
// a version no project file would accept is left out.
func TestDetectStaysInTheRepository(t *testing.T) {
	outside := files(t, map[string]string{"package.json": `{"vite": "7"}`, ".tool-versions": "just 9.9.9\n"})
	dir := files(t, map[string]string{
		"$(touch x)/go.mod": "module x\n",
		".tool-versions":    "just 1.0\" && evil\nmytool 1.2.3\n",
	})
	for _, name := range []string{"package.json", "linked"} {
		target := filepath.Join(outside, "package.json")
		if name == "linked" {
			target = outside
		}
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	d, f := detect(t, dir)
	if len(f.Packs) != 0 || len(f.Ports) != 0 || f.Setup != "" {
		t.Errorf("followed something it shouldn't have:\n%s", d.Render())
	}
	if f.Tools["mytool"] != "1.2.3" || len(f.Tools) != 1 {
		t.Errorf("tools %v", f.Tools)
	}
	if _, err := Detect(filepath.Join(dir, "missing"), packOf); err == nil {
		t.Error("a missing directory should fail")
	}
}
