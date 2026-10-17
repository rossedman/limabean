package templates

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rossedman/limabean/internal/pins"
)

// CorePack is installed in every VM, by layer-base.yaml itself.
const CorePack = "core"

// Pack is a set of tools a VM can get: a mise TOML (packs/<name>.toml),
// and optionally system packages (packs/<name>.apt, one per line).
type Pack struct {
	Name        string
	Description string   // the TOML's first comment line
	Tools       []string // mise tools, as pinned
	Apt         []string // apt packages
	TOML        string   // the file, as written into the VM
}

var packNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Packs returns the packs in root/packs and the user's own in
// root/user-packs (which win over built-in ones of the same name), by name.
func Packs(root string) ([]Pack, error) {
	builtin, _ := filepath.Glob(filepath.Join(root, "packs", "*.toml"))
	user, _ := filepath.Glob(filepath.Join(root, "user-packs", "*.toml"))
	var packs []Pack
	for _, f := range append(builtin, user...) {
		name := strings.TrimSuffix(filepath.Base(f), ".toml")
		if !packNameRE.MatchString(name) {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		p := Pack{Name: name, TOML: string(b)}
		for _, pin := range pins.Parse(p.TOML) {
			p.Tools = append(p.Tools, pin.Tool)
		}
		sc := bufio.NewScanner(strings.NewReader(p.TOML))
		if sc.Scan() {
			p.Description = strings.TrimSpace(strings.TrimPrefix(sc.Text(), "#"))
		}
		if b, err := os.ReadFile(strings.TrimSuffix(f, ".toml") + ".apt"); err == nil {
			p.Apt = aptPackages(string(b))
		}
		// A user pack replaces a built-in one of its name.
		packs = slices.DeleteFunc(packs, func(q Pack) bool { return q.Name == name })
		packs = append(packs, p)
	}
	slices.SortFunc(packs, func(a, b Pack) int { return strings.Compare(a.Name, b.Name) })
	return packs, nil
}

// aptPackages reads an apt list: package names, with # comments.
func aptPackages(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line, _, _ = strings.Cut(line, "#")
		out = append(out, strings.Fields(line)...)
	}
	return out
}

// AptList renders the pack's apt packages as the VM reads them.
func (p Pack) AptList() string {
	if len(p.Apt) == 0 {
		return ""
	}
	return strings.Join(p.Apt, "\n") + "\n"
}

// Contents describes what a pack installs, for listings.
func (p Pack) Contents() string {
	return strings.Join(append(slices.Clone(p.Tools), p.Apt...), ", ")
}

// ResolvePacks turns a default set and additions and removals into the
// sorted packs a VM gets (core isn't listed: every VM has it). Unknown
// names are an error that lists the known ones.
func ResolvePacks(all []Pack, defaults, with, without []string) ([]string, error) {
	known := map[string]bool{}
	var names []string
	for _, p := range all {
		known[p.Name] = true
		if p.Name != CorePack {
			names = append(names, p.Name)
		}
	}
	check := func(list []string) error {
		for _, n := range list {
			if !known[n] || n == CorePack {
				if n == CorePack {
					return fmt.Errorf("%q is in every VM; it can't be added or removed", n)
				}
				return fmt.Errorf("unknown pack %q (available: %s)", n, strings.Join(names, ", "))
			}
		}
		return nil
	}
	defaults = slices.DeleteFunc(slices.Clone(defaults), func(n string) bool { return n == CorePack })
	for _, list := range [][]string{defaults, with, without} {
		if err := check(list); err != nil {
			return nil, err
		}
	}
	set := append(slices.Clone(defaults), with...)
	set = slices.DeleteFunc(set, func(n string) bool { return slices.Contains(without, n) })
	slices.Sort(set)
	return slices.Compact(set), nil
}

// SplitList splits a comma- or space-separated list of names.
func SplitList(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' })
}
