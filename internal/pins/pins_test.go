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
	out := Rewrite(sample, map[string]string{"bat": "0.27.0", "go:github.com/posener/complete/v2/gocomplete": "2.2.0", "helm": "4.4.0", "FOO": "9"})
	want := strings.NewReplacer(
		`bat = "0.26.1"`, `bat = "0.27.0"`,
		`gocomplete" = "2.1.0"`, `gocomplete" = "2.2.0"`,
		`helm   =   "4.3.1"`, `helm   =   "4.4.0"`,
	).Replace(sample)
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// The real base.toml parses, and rewriting with its own versions is a no-op.
func TestRealBaseTOML(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "templates", "mise", "base.toml"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	pins := Parse(src)
	if len(pins) < 20 {
		t.Fatalf("only %d pins in base.toml", len(pins))
	}
	same := map[string]string{}
	for _, p := range pins {
		if Series(p.Version) == "" {
			t.Errorf("%s: version %q isn't numeric", p.Tool, p.Version)
		}
		same[p.Tool] = p.Version
	}
	if Rewrite(src, same) != src {
		t.Error("rewriting with the same versions changed the file")
	}
}

func TestSeries(t *testing.T) {
	for v, want := range map[string]string{"4.3.1": "4", "v1.27.2": "1", "0.26.1": "0.26", "0": "0", "latest": "", "": ""} {
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
