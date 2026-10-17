// Package pins reads and rewrites the pinned tool versions in a mise TOML
// file (templates/mise/*.toml), line by line, so comments and layout are
// kept: only the version strings change.
package pins

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Pin is one `tool = "version"` line under [tools].
type Pin struct {
	Tool    string // as mise names it, e.g. go or "go:github.com/x/y"
	Version string
	Line    int // 0-based
}

// pinRE matches a tool line: a bare or quoted key, then a quoted version,
// then anything (usually a comment).
var pinRE = regexp.MustCompile(`^(\s*)("[^"]+"|[A-Za-z0-9_.-]+)(\s*=\s*)"([^"]*)"(.*)$`)

// Parse returns the pins in a mise TOML file's [tools] table.
func Parse(src string) []Pin {
	var pins []Pin
	inTools := false
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inTools = trimmed == "[tools]"
			continue
		}
		if !inTools {
			continue
		}
		if m := pinRE.FindStringSubmatch(line); m != nil {
			pins = append(pins, Pin{Tool: strings.Trim(m[2], `"`), Version: m[4], Line: i})
		}
	}
	return pins
}

// Rewrite returns src with the given tools' versions replaced.
func Rewrite(src string, versions map[string]string) string {
	lines := strings.Split(src, "\n")
	for _, p := range Parse(src) {
		v, ok := versions[p.Tool]
		if !ok {
			continue
		}
		m := pinRE.FindStringSubmatch(lines[p.Line])
		lines[p.Line] = m[1] + m[2] + m[3] + strconv.Quote(v) + m[5]
	}
	return strings.Join(lines, "\n")
}

// Series is the range an upgrade stays in unless majors are allowed: the
// major version, or major.minor below 1.0, where minors can break. It's
// what `mise latest tool@<series>` takes. "" for versions it can't read.
func Series(version string) string {
	parts := strings.SplitN(strings.TrimPrefix(version, "v"), ".", 3)
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return ""
	}
	if parts[0] == "0" && len(parts) > 1 {
		return "0." + parts[1]
	}
	return parts[0]
}

// Newer reports whether version a is newer than b, comparing dot-separated
// numbers (a pre-release suffix sorts before its release).
func Newer(a, b string) bool {
	return compare(a, b) > 0
}

func compare(a, b string) int {
	split := func(v string) ([]string, string) {
		v = strings.TrimPrefix(v, "v")
		main, pre, _ := strings.Cut(v, "-")
		return strings.Split(main, "."), pre
	}
	pa, prea := split(a)
	pb, preb := split(b)
	for i := 0; i < max(len(pa), len(pb)); i++ {
		x, y := 0, 0
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	switch {
	case prea == preb:
		return 0
	case prea == "":
		return 1
	case preb == "":
		return -1
	}
	return strings.Compare(prea, preb)
}

// String formats a pin for messages.
func (p Pin) String() string { return fmt.Sprintf("%s %s", p.Tool, p.Version) }
