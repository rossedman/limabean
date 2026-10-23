package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzParseListening feeds parseListening arbitrary `ss -ltnpH` output from
// a VM, seeded with the real output from the command tests.
func FuzzParseListening(f *testing.F) {
	f.Add(ssOutput)
	for line := range strings.SplitSeq(ssOutput, "\n") {
		f.Add(line)
	}
	f.Add("garbage\nLISTEN 0 1 nocolon x\nLISTEN 0 1 1.2.3.4:x y\n")
	f.Add(`LISTEN 0      511         [::1]:8080     [::]:* users:(("node",pid=7,fd=20),("node",pid=8,fd=20))`)
	f.Fuzz(func(t *testing.T, out string) {
		list := parseListening(out)
		for i, p := range list {
			if p.Port < 1 || p.Port > 65535 {
				t.Fatalf("port %d out of range, from %q", p.Port, out)
			}
			if p.Port == 22 {
				t.Fatalf("listed SSH's port, from %q", out)
			}
			if i > 0 && list[i-1].Port >= p.Port {
				t.Fatalf("ports not sorted and unique: %v", list)
			}
			if strings.ContainsAny(p.Process, "\"\n") {
				t.Fatalf("process %q", p.Process)
			}
			if p.URL != "" || p.Forwarded {
				t.Fatalf("parseListening filled in %+v", p)
			}
		}
	})
}

// shellMeta are characters a shell, ssh or limactl would treat specially.
const shellMeta = " \t\n\r\v\f\x00/\\;&|$`'\"<>(){}[]*?!~#=%^,:@+"

// nameSeeds are names and attacks from the tests: flags, paths and shell.
var nameSeeds = []string{
	"dev", "agent", "my-app", "a", "--force", "-x", "", " dev", "dev ", "a/b", "../x",
	"x;rm -rf ~", "$(id)", "`id`", "a\x00b", "a\nb", imagePrefix + "deadbeef-x", "Dev2", "v1.2_rc", ".", "..",
}

func checkAccepted(t *testing.T, what, s, extra string) {
	t.Helper()
	if s == "" || s[0] == '-' || s[0] == '.' || s[0] == '_' {
		t.Fatalf("%s accepted %q", what, s)
	}
	if strings.ContainsAny(s, shellMeta) {
		t.Fatalf("%s accepted %q, with a shell or path character", what, s)
	}
	for _, r := range s {
		if !isAlnum(r) && r != '-' && !strings.ContainsRune(extra, r) {
			t.Fatalf("%s accepted %q, with %q", what, s, r)
		}
	}
}

func isAlnum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// plainName reports whether s is letters, digits and the extra characters,
// starting with a letter or digit: what the guards promise to accept.
func plainName(s, extra string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !isAlnum(r) && (i == 0 || (r != '-' && !strings.ContainsRune(extra, r))) {
			return false
		}
	}
	return true
}

// FuzzValidVMName checks the VM name guard accepts exactly letters, digits
// and dashes, starting with a letter or digit, and not an image's name.
func FuzzValidVMName(f *testing.F) {
	for _, s := range nameSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		err := validVMName(name)
		if want := plainName(name, "") && !isImage(name); (err == nil) != want {
			t.Fatalf("validVMName(%q) = %v, want accepted %v", name, err, want)
		}
		if err == nil {
			checkAccepted(t, "validVMName", name, "")
		}
	})
}

// FuzzValidSnapshotTag checks the snapshot tag guard: letters, digits,
// dots, dashes and underscores, starting with a letter or digit, so a tag
// is one path component and never a flag.
func FuzzValidSnapshotTag(f *testing.F) {
	for _, s := range nameSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tag string) {
		err := validSnapshotTag(tag)
		if want := plainName(tag, "._"); (err == nil) != want {
			t.Fatalf("validSnapshotTag(%q) = %v, want accepted %v", tag, err, want)
		}
		if err != nil {
			if !strings.Contains(err.Error(), "use letters, digits") {
				t.Fatalf("validSnapshotTag(%q): %v", tag, err)
			}
			return
		}
		checkAccepted(t, "validSnapshotTag", tag, "._")
		if tag == "." || tag == ".." || filepath.Base(tag) != tag || filepath.Join("snapshots", tag) != "snapshots/"+tag {
			t.Fatalf("validSnapshotTag accepted %q, which isn't one path component", tag)
		}
	})
}

// FuzzCheckNames checks MCP's name guard: the first name is required, the
// rest are optional, and every non-empty one passes validVMName.
func FuzzCheckNames(f *testing.F) {
	for i, s := range nameSeeds {
		f.Add(s, nameSeeds[(i+3)%len(nameSeeds)])
	}
	f.Fuzz(func(t *testing.T, vm, env string) {
		err := checkNames(vm, env)
		want := validVMName(vm) == nil && (env == "" || validVMName(env) == nil)
		if (err == nil) != want {
			t.Fatalf("checkNames(%q, %q) = %v, want accepted %v", vm, env, err, want)
		}
		if err != nil {
			return
		}
		checkAccepted(t, "checkNames", vm, "")
		if env != "" {
			checkAccepted(t, "checkNames", env, "")
		}
	})
}

// FuzzExpandAllow feeds expandAllow comma-separated allowlist items: every
// domain it returns is a plain lowercase domain name.
func FuzzExpandAllow(f *testing.F) {
	for _, s := range []string{
		"GitHub,*.Example.com,registry.npmjs.org", "github", "go,npm,python",
		"localhost", "https://pypi.org", "a..b", "-x.com", "x.com/path", "", "*.", " pypi.org ",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		items := strings.Split(in, ",")
		out, err := expandAllow(items)
		if err != nil {
			if out != nil || !strings.Contains(err.Error(), "isn't a domain or a preset") {
				t.Fatalf("expandAllow(%q) = %v, %v", items, out, err)
			}
			return
		}
		if len(out) < len(items) {
			t.Fatalf("expandAllow(%q) = %v lost items", items, out)
		}
		for _, d := range out {
			if d == "" || !netDomainRE.MatchString(d) || d != strings.ToLower(d) {
				t.Fatalf("expandAllow(%q) returned %q", items, d)
			}
		}
	})
}
