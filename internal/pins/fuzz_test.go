package pins

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// versionSamples are versions from the packs and the package's tests.
var versionSamples = []string{
	"15.2.0", "0.26.1", "4.3.1", "v1.27.2", "0", "latest", "", "21+35",
	"1.0.0-rc1", "1.0.0-rc2", "2.0", "temurin-25.0.4+101.0.LTS", "temurin-25.0.5+11",
	"zulu-25.0.1", "v0.9.0-beta.1+build.5", "1.10.0", "99999999999999999999.1",
}

// FuzzParse feeds Parse arbitrary mise TOML, starting from the package's
// sample and the real packs.
func FuzzParse(f *testing.F) {
	f.Add(sample)
	packs, _ := filepath.Glob(filepath.Join("..", "..", "templates", "packs", "*.toml"))
	for _, p := range packs {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(string(b))
		}
	}
	f.Fuzz(func(t *testing.T, src string) {
		lines := strings.Split(src, "\n")
		last := -1
		for _, p := range Parse(src) {
			if p.Line <= last || p.Line >= len(lines) {
				t.Fatalf("pin %+v: line out of order or range (%d lines)", p, len(lines))
			}
			last = p.Line
			if p.Tool == "" || strings.Contains(p.Tool, `"`) {
				t.Fatalf("pin %+v: tool name", p)
			}
			if strings.Contains(p.Version, `"`) || !strings.Contains(lines[p.Line], `"`+p.Version+`"`) {
				t.Fatalf("pin %+v isn't a quoted version on its line %q", p, lines[p.Line])
			}
			if strings.HasPrefix(strings.TrimSpace(lines[p.Line]), "[") {
				t.Fatalf("pin %+v on a table header", p)
			}
		}
	})
}

// FuzzVersions checks Prefix, Series and the version ordering on pairs of
// arbitrary versions.
func FuzzVersions(f *testing.F) {
	for i, a := range versionSamples {
		f.Add(a, versionSamples[(i+1)%len(versionSamples)])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		for _, v := range []string{a, b} {
			checkPrefixAndSeries(t, v)
		}
		if c := compareVersions(a, a); c != 0 {
			t.Fatalf("compareVersions(%q, itself) = %d", a, c)
		}
		if ab, ba := compareVersions(a, b), compareVersions(b, a); ab != -ba {
			t.Fatalf("compareVersions(%q, %q) = %d but the reverse is %d", a, b, ab, ba)
		}
		if compare(a, a) != 0 || Newer(a, a) {
			t.Fatalf("%q is newer than itself", a)
		}
		if ab, ba := compare(a, b), compare(b, a); (ab > 0) != (ba < 0) || (ab == 0) != (ba == 0) {
			t.Fatalf("compare(%q, %q) = %d but the reverse is %d", a, b, ab, ba)
		}
	})
}

// seriesRE is what a series is after its vendor prefix: a major version,
// or 0.minor.
var seriesRE = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

// checkPrefixAndSeries asserts what Prefix and Series promise: the prefix
// starts the version, followed by a dash, and the series is the prefix
// and the start of what follows it (without a leading v), beginning with
// a number: a major version, or 0.minor.
func checkPrefixAndSeries(t *testing.T, v string) {
	t.Helper()
	rest := v
	p := Prefix(v)
	if p != "" {
		if !strings.HasPrefix(v, p+"-") {
			t.Fatalf("Prefix(%q) = %q doesn't start it", v, p)
		}
		rest = strings.TrimPrefix(v, p+"-")
		if rest == "" || rest[0] < '0' || rest[0] > '9' {
			t.Fatalf("Prefix(%q) = %q isn't followed by a number", v, p)
		}
	}
	s := Series(v)
	if s == "" {
		return
	}
	if p != "" {
		if !strings.HasPrefix(s, p+"-") {
			t.Fatalf("Series(%q) = %q drops its prefix %q", v, s, p)
		}
		s = strings.TrimPrefix(s, p+"-")
	}
	rest = strings.TrimPrefix(rest, "v")
	if s == "" || !strings.HasPrefix(rest, s) {
		t.Fatalf("Series(%q) = %q isn't the start of %q", v, Series(v), rest)
	}
	if !seriesRE.MatchString(s) || (strings.Contains(s, ".") && !strings.HasPrefix(s, "0.")) {
		t.Fatalf("Series(%q) = %q isn't a major version or 0.minor", v, Series(v))
	}
}
