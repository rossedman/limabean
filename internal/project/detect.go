package project

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rossedman/limabean/internal/pins"
)

// A Draft is a project file worked out from what's in a repository, for
// limabean init --project. Detect only reads files: it never runs the
// repository's code or follows a symlink out of it.
type Draft struct {
	Packs   []string          // sorted
	Tools   map[string]string // version pins no pack covers
	Ports   []int             // sorted
	Setup   []string          // commands, in order
	Start   []string
	Secrets []string // names from .env.example that look like credentials

	why   map[string][]string // pack → what suggested it
	notes []string            // comments for the top of the file
}

// Found lists what Detect recognised, one line each, for the user.
func (d *Draft) Found() []string {
	var out []string
	for _, p := range d.Packs {
		out = append(out, p+": "+strings.Join(d.why[p], ", "))
	}
	return append(out, d.notes...)
}

// Empty reports whether nothing was recognised.
func (d *Draft) Empty() bool {
	return len(d.Packs) == 0 && len(d.Tools) == 0 && len(d.Setup) == 0 && len(d.Start) == 0
}

// pack adds a pack to the draft, once, and records why it was suggested.
func (d *Draft) pack(name, why string) {
	if d.why == nil {
		d.why = map[string][]string{}
	}
	if !slices.Contains(d.Packs, name) {
		d.Packs = append(d.Packs, name)
	}
	if !slices.Contains(d.why[name], why) {
		d.why[name] = append(d.why[name], why)
	}
}

// port adds a port to the draft, once.
func (d *Draft) port(p int) {
	if !slices.Contains(d.Ports, p) {
		d.Ports = append(d.Ports, p)
	}
}

// in runs cmd in a subdirectory of the clone, or the clone itself.
func in(dir, cmd string) string {
	if dir == "" {
		return cmd
	}
	return "(cd " + dir + " && " + cmd + ")"
}

var dirRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)

// skipDirs are never looked into for a project's parts.
var skipDirs = map[string]bool{"node_modules": true, "vendor": true, "target": true, "dist": true, "build": true, "deps": true, "_build": true}

// Detect works out a draft project file from the files in dir and the
// directories directly under it, so a monorepo's web/ and api/ count too.
// packOf names the pack that installs a mise tool ("core" for one every
// VM has), or "".
func Detect(dir string, packOf func(tool string) string) (*Draft, error) {
	d := &Draft{Tools: map[string]string{}}
	parts := []string{""}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		// Names go into setup's cd, so only plain ones.
		if e.IsDir() && dirRE.MatchString(e.Name()) && !skipDirs[e.Name()] && len(parts) < 21 {
			parts = append(parts, e.Name())
		}
	}
	for _, sub := range parts {
		r := reader{root: dir, sub: sub}
		r.detect(d, packOf)
	}
	slices.Sort(d.Packs)
	slices.Sort(d.Ports)
	slices.Sort(d.Secrets)
	d.Secrets = slices.Compact(d.Secrets)
	return d, nil
}

// reader reads files in one directory of the repository: regular files
// only, at most 1 MiB, so a symlink can't point it anywhere else.
type reader struct{ root, sub string }

// path returns the path of a file in the reader's directory.
func (r reader) path(name string) string { return filepath.Join(r.root, r.sub, name) }

// has reports whether name is a regular file in the directory; a symlink
// doesn't count.
func (r reader) has(name string) bool {
	fi, err := os.Lstat(r.path(name))
	return err == nil && fi.Mode().IsRegular()
}

// read returns the first 1 MiB of a regular file in the directory, or "" if
// it isn't one or can't be read.
func (r reader) read(name string) string {
	if !r.has(name) {
		return ""
	}
	f, err := os.Open(r.path(name))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	var b bytes.Buffer
	_, _ = b.ReadFrom(io.LimitReader(f, 1<<20))
	return b.String()
}

// glob reports whether a regular file in the directory matches pattern.
// The directory's own path is never a pattern, whatever it contains.
func (r reader) glob(pattern string) bool {
	entries, _ := os.ReadDir(filepath.Join(r.root, r.sub))
	for _, e := range entries {
		if ok, _ := filepath.Match(pattern, e.Name()); ok && e.Type().IsRegular() {
			return true
		}
	}
	return false
}

// where says which file suggested something: "go.mod", or "api/go.mod".
func (r reader) where(name string) string { return filepath.ToSlash(filepath.Join(r.sub, name)) }

// detect adds to the draft what this directory's files suggest: packs, setup
// commands, start commands, ports and secrets.
func (r reader) detect(d *Draft, packOf func(string) string) {
	if r.has("go.mod") {
		d.pack("go", r.where("go.mod"))
		d.Setup = append(d.Setup, in(r.sub, "go mod download"))
	}
	if r.has("package.json") {
		d.pack("node", r.where("package.json"))
		switch {
		case r.has("pnpm-lock.yaml"):
			d.Setup = append(d.Setup, in(r.sub, "pnpm install --frozen-lockfile"))
		case r.has("bun.lock") || r.has("bun.lockb"):
			d.Setup = append(d.Setup, in(r.sub, "bun install"))
		case r.has("yarn.lock"):
			d.Setup = append(d.Setup, in(r.sub, "npx --yes yarn install"))
		case r.has("package-lock.json"):
			d.Setup = append(d.Setup, in(r.sub, "npm ci"))
		default:
			d.Setup = append(d.Setup, in(r.sub, "npm install"))
		}
		pkg := r.read("package.json")
		for dep, port := range map[string]int{`"vite"`: 5173, `"next"`: 3000, `"nuxt"`: 3000, `"astro"`: 4321, `"@remix-run/dev"`: 3000, `"storybook"`: 6006} {
			if strings.Contains(pkg, dep) {
				d.port(port)
			}
		}
	}
	python := ""
	switch {
	case r.has("uv.lock"):
		python, d.Setup = "uv.lock", append(d.Setup, in(r.sub, "uv sync"))
	case r.has("poetry.lock"):
		python, d.Setup = "poetry.lock", append(d.Setup, in(r.sub, "uvx poetry install"))
	case r.has("Pipfile"):
		python, d.Setup = "Pipfile", append(d.Setup, in(r.sub, "uvx pipenv install"))
	case r.has("pyproject.toml"):
		python, d.Setup = "pyproject.toml", append(d.Setup, in(r.sub, "uv sync"))
	case r.has("requirements.txt"):
		python, d.Setup = "requirements.txt", append(d.Setup, in(r.sub, "uv venv && uv pip install -r requirements.txt"))
	}
	if python != "" {
		d.pack("python", r.where(python))
		deps := strings.ToLower(r.read("pyproject.toml") + r.read("requirements.txt") + r.read("Pipfile"))
		switch {
		case r.has("manage.py") || strings.Contains(deps, "django"):
			d.port(8000)
		case strings.Contains(deps, "fastapi") || strings.Contains(deps, "uvicorn"):
			d.port(8000)
		case strings.Contains(deps, "flask"):
			d.port(5000)
		}
	}
	if r.has("Cargo.toml") {
		d.pack("rust", r.where("Cargo.toml"))
		d.Setup = append(d.Setup, in(r.sub, "cargo fetch"))
	}
	if r.has("Gemfile") {
		d.pack("ruby", r.where("Gemfile"))
		d.Setup = append(d.Setup, in(r.sub, "bundle install"))
		if strings.Contains(r.read("Gemfile"), "rails") {
			d.port(3000)
		}
	}
	if r.has("mix.exs") {
		d.pack("elixir", r.where("mix.exs"))
		d.Setup = append(d.Setup, in(r.sub, "mix local.hex --force && mix deps.get"))
		if strings.Contains(r.read("mix.exs"), ":phoenix") {
			d.port(4000)
		}
	}
	switch {
	case r.has("pom.xml"):
		d.pack("java", r.where("pom.xml"))
		mvn := "mvn"
		if r.has("mvnw") {
			mvn = "./mvnw"
		}
		d.Setup = append(d.Setup, in(r.sub, mvn+" -q dependency:go-offline"))
		if strings.Contains(r.read("pom.xml"), "spring-boot") {
			d.port(8080)
		}
	case r.has("build.gradle") || r.has("build.gradle.kts"):
		d.pack("java", r.where(firstFile(r, "build.gradle.kts", "build.gradle")))
		gradle := "gradle"
		if r.has("gradlew") {
			gradle = "./gradlew"
		}
		d.Setup = append(d.Setup, in(r.sub, gradle+" --quiet dependencies"))
		if strings.Contains(r.read("build.gradle")+r.read("build.gradle.kts"), "spring") {
			d.port(8080)
		}
	}
	if r.glob("*.csproj") || r.glob("*.sln") || r.glob("*.fsproj") {
		d.pack("dotnet", r.where("a .NET project"))
		d.Setup = append(d.Setup, in(r.sub, "dotnet restore"))
	}
	if r.glob("*.tf") {
		d.pack("infra", r.where("*.tf"))
	}
	for _, f := range []string{"Chart.yaml", "kustomization.yaml", "skaffold.yaml", "Tiltfile"} {
		if r.has(f) {
			d.pack("kubernetes", r.where(f))
			break
		}
	}
	for _, f := range []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"} {
		if r.has(f) {
			d.Start = append(d.Start, in(r.sub, "docker compose up -d"))
			d.notes = append(d.notes, "start: "+r.where(f))
			break
		}
	}
	r.toolVersions(d, packOf)
	for _, f := range []string{".env.example", ".env.sample", ".env.template"} {
		d.Secrets = append(d.Secrets, credentialNames(r.read(f))...)
	}
}

// firstFile returns the first of names that exists in the directory, or the
// first name when none does.
func firstFile(r reader, names ...string) string {
	for _, n := range names {
		if r.has(n) {
			return n
		}
	}
	return names[0]
}

// asdfNames are .tool-versions names mise knows by another.
var asdfNames = map[string]string{"nodejs": "node", "golang": "go"}

// toolVersions reads .tool-versions and mise's own files. A tool a pack
// installs adds the pack; any other goes under tools:, if its name and
// version are ones a project file accepts.
func (r reader) toolVersions(d *Draft, packOf func(string) string) {
	found := map[string]string{}
	source := ""
	sc := bufio.NewScanner(strings.NewReader(r.read(".tool-versions")))
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if f := strings.Fields(line); len(f) >= 2 {
			found[f[0]] = f[1]
			source = r.where(".tool-versions")
		}
	}
	for _, name := range []string{"mise.toml", ".mise.toml"} {
		for _, p := range pins.Parse(r.read(name)) {
			found[p.Tool] = p.Version
			source = r.where(name)
		}
	}
	for _, tool := range slices.Sorted(maps.Keys(found)) {
		version := found[tool]
		name := tool
		if n, ok := asdfNames[tool]; ok {
			name = n
		}
		p := packOf(name)
		if p == "core" { // in every VM already
			d.notes = append(d.notes, fmt.Sprintf("%s pins %s %s; every VM has it (core)", source, name, version))
			continue
		}
		if p != "" {
			d.pack(p, source)
			d.notes = append(d.notes, fmt.Sprintf("%s pins %s %s; the %s pack has its own version", source, name, version, p))
			continue
		}
		// Only pins: not system, latest or lts.
		if toolRE.MatchString(name) && versionRE.MatchString(version) && strings.ContainsAny(version, "0123456789") {
			d.Tools[name] = version
			d.notes = append(d.notes, "tools: "+name+" from "+source)
		}
	}
}

var credentialRE = regexp.MustCompile(`(?m)^\s*(?:export\s+)?([A-Z][A-Z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIALS?)[A-Z0-9_]*)\s*=`)

// credentialNames are the variables in an example env file that look like
// credentials: the ones a VM would get as secrets.
func credentialNames(s string) []string {
	var names []string
	for _, m := range credentialRE.FindAllStringSubmatch(s, -1) {
		if secretRE.MatchString(m[1]) && len(m[1]) <= 100 {
			names = append(names, m[1])
		}
	}
	return names
}

// Render writes the draft as a commented project file.
func (d *Draft) Render() string {
	var b strings.Builder
	b.WriteString(SchemaComment)
	b.WriteString("# Written by limabean init --project from what's in this repository.\n")
	b.WriteString("# Check it, then: limabean up   (every key: limabean up --help)\n")
	if d.Empty() {
		b.WriteString("# Nothing here was recognised; limabean recipes lists starting points.\n")
	}
	if len(d.Packs) > 0 {
		fmt.Fprintf(&b, "packs: [%s]\n", strings.Join(d.Packs, ", "))
	}
	if len(d.Tools) > 0 {
		b.WriteString("tools:\n")
		for _, name := range slices.Sorted(maps.Keys(d.Tools)) {
			fmt.Fprintf(&b, "  %q: %q\n", name, d.Tools[name])
		}
	}
	if len(d.Ports) > 0 {
		var ps []string
		for _, p := range d.Ports {
			ps = append(ps, fmt.Sprint(p))
		}
		fmt.Fprintf(&b, "ports: [%s]\n", strings.Join(ps, ", "))
	}
	if len(d.Secrets) > 0 {
		b.WriteString("# Secrets from .env.example; set each up first with limabean secrets set NAME:\n")
		fmt.Fprintf(&b, "# secrets: [%s]\n", strings.Join(d.Secrets, ", "))
	}
	block := func(key string, lines []string) {
		if len(lines) == 0 {
			return
		}
		b.WriteString(key + ": |\n")
		for _, l := range lines {
			b.WriteString("  " + l + "\n")
		}
	}
	block("setup", d.Setup)
	block("start", d.Start)
	return b.String()
}
