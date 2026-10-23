// Package secrets manages the environment variables limabean installs in VMs.
//
// On the Mac, secrets are described by references, never stored as values,
// in two kinds of files with one NAME=reference per line:
//
//	~/.config/limabean/secrets.env         every VM
//	~/.config/limabean/secrets.<vm>.env    one VM; overrides the shared file
//
// A reference is resolved on the Mac when secrets are synced:
//
//	keychain:<service>      a macOS Keychain item (account: the current user)
//	op://vault/item/field   a 1Password item, read with the op CLI
//	env:<VAR>               a variable from the environment limabean runs in
//	cmd:<command>           a command's output, run with sh
//	anything else           used as is (for settings, never for secrets)
package secrets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rossedman/limabean/internal/atomic"
)

// Secret is one NAME=reference line and the file it came from.
type Secret struct {
	Name   string
	Ref    string
	Source string // file name, e.g. "secrets.env"
}

// KeychainPrefix starts the service name of Keychain items created by
// `limabean secrets set`, e.g. "limabean-secret-OPENAI_API_KEY".
const KeychainPrefix = "limabean-secret-"

var nameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidName reports whether s can be an environment variable name.
func ValidName(s string) bool { return nameRE.MatchString(s) }

// SharedFile and VMFile return the reference files in dir.
func SharedFile(dir string) string { return filepath.Join(dir, "secrets.env") }

func VMFile(dir, vm string) string { return filepath.Join(dir, "secrets."+vm+".env") }

// Parse reads NAME=reference lines. Blank lines, comments and a leading
// "export " are allowed; a reference may be wrapped in matching quotes. Lines
// that don't parse are returned as errors and skipped.
func Parse(r io.Reader, source string) ([]Secret, []error) {
	var out []Secret
	var errs []error
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, ref, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || !ValidName(name) {
			errs = append(errs, fmt.Errorf("%s line %d: expected NAME=reference", source, n))
			continue
		}
		out = append(out, Secret{Name: name, Ref: unquote(strings.TrimSpace(ref)), Source: source})
	}
	if err := sc.Err(); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", source, err))
	}
	return out, errs
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// Merge combines lists in order; a later entry replaces an earlier one with
// the same name and takes its place in the order.
func Merge(lists ...[]Secret) []Secret {
	var order []string
	byName := map[string]Secret{}
	for _, list := range lists {
		for _, s := range list {
			if _, seen := byName[s.Name]; !seen {
				order = append(order, s.Name)
			}
			byName[s.Name] = s
		}
	}
	out := make([]Secret, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name])
	}
	return out
}

// Load returns the secrets for a VM: the shared file merged with the VM's
// own file. With vm == "" it returns the shared file alone. Missing files
// are fine.
func Load(dir, vm string) ([]Secret, []error) {
	files := []string{SharedFile(dir)}
	if vm != "" {
		files = append(files, VMFile(dir, vm))
	}
	var lists [][]Secret
	var errs []error
	for _, path := range files {
		list, e := parseFile(path)
		lists = append(lists, list)
		errs = append(errs, e...)
	}
	return Merge(lists...), errs
}

// LoadFile returns the entries of a single reference file.
func LoadFile(path string) ([]Secret, []error) { return parseFile(path) }

func parseFile(path string) ([]Secret, []error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{err}
	}
	defer func() { _ = f.Close() }()
	return Parse(f, filepath.Base(path))
}

// Kind describes how a reference is resolved, for listings. It never
// includes a plain value, which might be a secret pasted by mistake.
func Kind(ref string) string {
	switch {
	case strings.HasPrefix(ref, "keychain:"):
		return "Keychain " + strings.TrimPrefix(ref, "keychain:")
	case strings.HasPrefix(ref, "op://"):
		return "1Password " + strings.TrimPrefix(ref, "op://")
	case strings.HasPrefix(ref, "env:"):
		return "$" + strings.TrimPrefix(ref, "env:") + " on the Mac"
	case strings.HasPrefix(ref, "cmd:"):
		return "command: " + strings.TrimPrefix(ref, "cmd:")
	default:
		return "plain value"
	}
}

// fileHeader starts every reference file limabean creates.
const fileHeader = `# limabean secrets: NAME=reference, one per line. See docs/secrets.md.
#   keychain:<service>   op://vault/item/field   env:<VAR>   cmd:<command>
`

// Put sets NAME's reference in a file, keeping every other line as it is, or
// removes NAME when ref is "". The file is created with a header if needed
// and always left with mode 0600.
func Put(path, name, ref string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, err = []byte(fileHeader), nil
	}
	if err != nil {
		return err
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "export ")
		if n, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(n) == name {
			continue
		}
		kept = append(kept, line)
	}
	if ref != "" {
		kept = append(kept, name+"="+ref)
	}
	if err := atomic.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// Value is a resolved secret.
type Value struct {
	Name  string
	Value string
}

// Render writes the VM's secrets file: one `export NAME='value'` line per
// secret, single-quoted so that any value is taken literally by sh, bash and
// zsh.
func Render(values []Value) string {
	var b strings.Builder
	b.WriteString("# Written by `limabean secrets sync` from references on your Mac.\n")
	for _, v := range values {
		fmt.Fprintf(&b, "export %s=%s\n", v.Name, ShellQuote(v.Value))
	}
	return b.String()
}

// ShellQuote wraps s in single quotes, writing each ' as '\”.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
