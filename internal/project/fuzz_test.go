package project

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// projectSamples are .limabean.yaml bodies from the package's tests: files
// Parse accepts and files it must refuse.
var projectSamples = []string{
	"",
	`
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
`,
	`claude:
  marketplaces:
    - marketplace: rossedman/guardrails
      plugins: [guardrails@guardrails]
    - marketplace: https://git.example.com/team/plugins.git
      plugins: [review@team, deploy@team]
`,
	"name: api\nenv: node\ntools: {terraform: 1.9.8, aqua:cli/cli: 2.60.0}\napt: [libpq-dev]\nvars: {APP_ENV: dev}\nstart: npm run dev\n",
	"pakcs: [rust]",
	"name: bad_name",
	"env: ../x",
	"packs: [--with]",
	"secrets: [X=cmd:id]",
	"cpus: -1",
	"ports: [70000]",
	"repo: --upload-pack=evil",
	"network: {allow: github}",
	"vars: {PATH: /evil}",
	"vars: {LD_PRELOAD: /x.so}",
	"vars: {TOKEN: x}\nsecrets: [TOKEN]",
	"vars: {A: \"one\\ntwo\"}",
	"tools: {-rf: 1}",
	"tools: {go: \"1.23\\n[tools]\"}",
	"apt: ['libpq-dev; curl x | sh']",
}

// FuzzParse feeds Parse untrusted .limabean.yaml contents. Whatever it
// accepts must pass the package's own checks, and writing it back out and
// parsing it again must give the same file.
func FuzzParse(f *testing.F) {
	for _, s := range projectSamples {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		const path = "/repo/My App/.limabean.yaml"
		got, err := Parse(b, path)
		if err != nil {
			if got != nil {
				t.Fatalf("Parse returned a file with an error: %v", err)
			}
			if !strings.HasPrefix(err.Error(), path+": ") {
				t.Fatalf("error doesn't name the file: %v", err)
			}
			return
		}
		if err := got.check(); err != nil {
			t.Fatalf("accepted a file that fails check: %v", err)
		}
		if !nameRE.MatchString(got.Name) || strings.HasPrefix(got.Name, "-") {
			t.Fatalf("accepted name %q", got.Name)
		}
		if got.Env != "" && !nameRE.MatchString(got.Env) {
			t.Fatalf("accepted env %q", got.Env)
		}
		for _, p := range slices.Concat(got.Packs, got.Without) {
			if !nameRE.MatchString(p) {
				t.Fatalf("accepted pack %q", p)
			}
		}
		for _, s := range got.Secrets {
			if !secretRE.MatchString(s) {
				t.Fatalf("accepted secret name %q", s)
			}
		}
		if strings.HasPrefix(got.Repo, "-") {
			t.Fatalf("accepted repo %q", got.Repo)
		}
		out, err := yaml.Marshal(got)
		if err != nil {
			t.Fatalf("marshalling an accepted file: %v", err)
		}
		again, err := Parse(out, path)
		if err != nil {
			t.Fatalf("re-parsing %q: %v", out, err)
		}
		out2, err := yaml.Marshal(again)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != string(out2) {
			t.Fatalf("re-parsing isn't stable:\n%s\n---\n%s", out, out2)
		}
	})
}
