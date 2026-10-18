package pins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# comment
[settings]
experimental = "true"

[tools]
# shell
ripgrep = "15.2.0"
bat = "0.26.1"   # pager in the host gitconfig
"go:github.com/posener/complete/v2/gocomplete" = "2.1.0"   # completion
  helm   =   "4.3.1"
"pipx:pyright" = { version = "1.1.414", extras = "nodejs" }   # options
other = { extras = "x" }
not a pin

[env]
FOO = "1.2.3"
`

func TestParse(t *testing.T) {
	got := Parse(sample)
	want := []Pin{
		{"ripgrep", "15.2.0", 6},
		{"bat", "0.26.1", 7},
		{"go:github.com/posener/complete/v2/gocomplete", "2.1.0", 8},
		{"helm", "4.3.1", 9},
		{"pipx:pyright", "1.1.414", 10},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pin %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[0].String() != "ripgrep 15.2.0" {
		t.Errorf("String = %q", got[0].String())
	}
}

func TestRewriteKeepsLayout(t *testing.T) {
	out := Rewrite(sample, map[string]string{"bat": "0.27.0", "go:github.com/posener/complete/v2/gocomplete": "2.2.0", "helm": "4.4.0", "pipx:pyright": "1.1.420", "FOO": "9"})
	want := strings.NewReplacer(
		`bat = "0.26.1"`, `bat = "0.27.0"`,
		`gocomplete" = "2.1.0"`, `gocomplete" = "2.2.0"`,
		`helm   =   "4.3.1"`, `helm   =   "4.4.0"`,
		`{ version = "1.1.414", extras`, `{ version = "1.1.420", extras`,
	).Replace(sample)
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// The real packs parse, pin versions bump can follow, and rewriting with
// their own versions is a no-op.
func TestRealPacks(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "templates", "packs", "*.toml"))
	if err != nil || len(files) < 10 {
		t.Fatalf("packs: %v %v", files, err)
	}
	total := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		same := map[string]string{}
		for _, p := range Parse(src) {
			if Series(p.Version) == "" {
				t.Errorf("%s %s: version %q has no series bump can follow", filepath.Base(f), p.Tool, p.Version)
			}
			same[p.Tool] = p.Version
			total++
		}
		if Rewrite(src, same) != src {
			t.Errorf("%s: rewriting with the same versions changed the file", f)
		}
	}
	if total < 40 {
		t.Errorf("only %d pins across the packs", total)
	}
}

func TestPrefixedVersions(t *testing.T) {
	v := "temurin-25.0.4+101.0.LTS"
	if Prefix(v) != "temurin" || Series(v) != "temurin-25" || Prefix("25.0.1") != "" {
		t.Errorf("Prefix/Series of %s: %q %q", v, Prefix(v), Series(v))
	}
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"temurin-25.0.5+11", "temurin-25.0.4+101.0.LTS", true},
		{"temurin-25.0.4+101.0.LTS", "temurin-25.0.4+100", true},
		{"temurin-25.0.4+101", "temurin-25.0.4+101", false},
		{"temurin-27.0.0+35", "temurin-25.0.4+101.0.LTS", true},
		{"zulu-25.0.1", "temurin-25.0.1", true}, // different vendors: ordered, never equal
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

func TestSeries(t *testing.T) {
	for v, want := range map[string]string{"4.3.1": "4", "v1.27.2": "1", "0.26.1": "0.26", "0": "0", "latest": "", "": "", "21+35": "21"} {
		if got := Series(v); got != want {
			t.Errorf("Series(%q) = %q, want %q", v, got, want)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"1.10.0", "1.9.9", true},
		{"1.9.9", "1.10.0", false},
		{"2.0.0", "2.0.0", false},
		{"2.0", "2.0.0", false},
		{"2.0.1", "2.0", true},
		{"v1.2.3", "1.2.2", true},
		{"1.0.0", "1.0.0-rc1", true},
		{"1.0.0-rc1", "1.0.0", false},
		{"1.0.0-rc2", "1.0.0-rc1", true},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
