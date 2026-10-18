package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"go.yaml.in/yaml/v3"

	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/templates"
	"github.com/rossedman/limabean/recipes"
)

var updateSchema = flag.Bool("update-schema", false, "rewrite docs/limabean.schema.json")

const schemaFile = "../../docs/limabean.schema.json"

// generatedSchema is the project file's schema, with the built-in packs and
// network presets editors offer.
func generatedSchema(t *testing.T) []byte {
	t.Helper()
	all, err := templates.Packs("../../templates")
	if err != nil {
		t.Fatal(err)
	}
	var packs []string
	for _, p := range all {
		if p.Name != templates.CorePack {
			packs = append(packs, p.Name)
		}
	}
	presets := mapKeys(netPresets)
	slices.Sort(presets)
	b, err := json.MarshalIndent(project.Schema(packs, presets, netDomainRE.String()), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

// The published schema is generated; after changing the project file, a
// pack or a preset, run: go test ./internal/cli -run TestSchema -update-schema
func TestSchemaIsCurrent(t *testing.T) {
	want := generatedSchema(t)
	if *updateSchema {
		if err := os.WriteFile(schemaFile, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("docs/limabean.schema.json is out of date: go test ./internal/cli -run TestSchema -update-schema")
	}
}

// What Load accepts, the schema accepts, and the other way round for the
// refusals that matter.
func TestSchemaAgreesWithLoad(t *testing.T) {
	var s jsonschema.Schema
	if err := json.Unmarshal(generatedSchema(t), &s); err != nil {
		t.Fatal(err)
	}
	resolved, err := s.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	validate := func(body []byte) error {
		var doc any
		if err := yaml.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		if doc == nil {
			doc = map[string]any{}
		}
		return resolved.Validate(doc)
	}

	files, _ := fs.Glob(recipes.Files, "*.yaml")
	for _, f := range files {
		body, _ := fs.ReadFile(recipes.Files, f)
		if err := validate(body); err != nil {
			t.Errorf("recipe %s: %v", f, err)
		}
	}
	full := `name: myapp
env: scratch
packs: [node, my-own-pack]
tools: {terraform: 1.9.8, go: 1.23, "npm:@biomejs/biome": 2.1.2}
apt: [libpq-dev]
cpus: 4
memory: 8
disk: 30
network: {allow: [github, "*.example.com"], locked: true}
secrets: [OPENAI_API_KEY]
ports: [3000]
vars: {NODE_ENV: development, WORKERS: 4, DEBUG: true}
setup: npm ci
start: npm run dev
claude:
  marketplaces:
    - marketplace: rossedman/guardrails
      plugins: [guardrails@guardrails]
`
	if err := validate([]byte(full)); err != nil {
		t.Errorf("a full file: %v", err)
	}
	for _, body := range []string{
		"enviroment: x",
		"vars: {PATH: /evil}",
		"vars: {LD_PRELOAD: x}",
		"tools: {go: '1 2'}",
		"apt: ['--allow-unauthenticated']",
		"ports: [70000]",
		"network: {allow: ['not a domain']}",
		"claude: {marketplaces: [{marketplace: x}]}",
		"secrets: [X=cmd:id]",
	} {
		if err := validate([]byte(body)); err == nil {
			t.Errorf("the schema accepts %q", body)
		}
		dir := t.TempDir()
		p := dir + "/" + project.FileName
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := project.Load(p); err == nil && !strings.Contains(body, "network") { // up checks domains
			t.Errorf("Load accepts %q", body)
		}
	}
}
