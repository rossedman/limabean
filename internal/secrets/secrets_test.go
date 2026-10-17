package secrets

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	in := `# a comment

OPENAI_API_KEY=op://Private/OpenAI/key
export NPM_TOKEN = keychain:npm-token
QUOTED="cmd:echo hi"
SINGLE='env:HOME'
REGION=us-east-1
EMPTY=
bad line
1BAD=x
`
	got, errs := Parse(strings.NewReader(in), "secrets.env")
	want := []Secret{
		{"OPENAI_API_KEY", "op://Private/OpenAI/key", "secrets.env"},
		{"NPM_TOKEN", "keychain:npm-token", "secrets.env"},
		{"QUOTED", "cmd:echo hi", "secrets.env"},
		{"SINGLE", "env:HOME", "secrets.env"},
		{"REGION", "us-east-1", "secrets.env"},
		{"EMPTY", "", "secrets.env"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d secrets, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("secret %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "secrets.env line 9") {
		t.Errorf("first error = %q, want it to name line 9", errs[0])
	}
}

func TestMergeLaterWinsKeepsFirstPosition(t *testing.T) {
	shared := []Secret{{"A", "1", "secrets.env"}, {"B", "2", "secrets.env"}}
	vm := []Secret{{"B", "vm", "secrets.dev.env"}, {"C", "3", "secrets.dev.env"}}
	got := Merge(shared, vm)
	want := []Secret{{"A", "1", "secrets.env"}, {"B", "vm", "secrets.dev.env"}, {"C", "3", "secrets.dev.env"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLoadMergesSharedAndVMFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, SharedFile(dir), "A=1\nB=2\n")
	write(t, VMFile(dir, "dev"), "B=dev\n")
	got, errs := Load(dir, "dev")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(got) != 2 || got[1].Ref != "dev" || got[1].Source != "secrets.dev.env" {
		t.Fatalf("got %+v", got)
	}
	shared, _ := Load(dir, "")
	if len(shared) != 2 || shared[1].Ref != "2" {
		t.Fatalf("shared only: got %+v", shared)
	}
	none, errs := Load(t.TempDir(), "dev")
	if len(none) != 0 || len(errs) != 0 {
		t.Fatalf("missing files should be empty, got %+v %v", none, errs)
	}
}

func TestPut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "secrets.env")
	if err := Put(path, "A", "keychain:a"); err != nil {
		t.Fatal(err)
	}
	if err := Put(path, "B", "env:B"); err != nil {
		t.Fatal(err)
	}
	if err := Put(path, "A", "op://v/i/f"); err != nil { // replace keeps one line
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.HasPrefix(got, "# limabean secrets") {
		t.Errorf("missing header:\n%s", got)
	}
	if strings.Count(got, "A=") != 1 || !strings.Contains(got, "A=op://v/i/f\n") || !strings.Contains(got, "B=env:B\n") {
		t.Errorf("unexpected content:\n%s", got)
	}
	if err := Put(path, "A", ""); err != nil { // remove
		t.Fatal(err)
	}
	if strings.Contains(read(t, path), "A=") {
		t.Errorf("A not removed:\n%s", read(t, path))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestPutKeepsUnrelatedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.env")
	write(t, path, "# mine\nexport A=1\nAB=2\n\n# end\n")
	if err := Put(path, "A", ""); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "# mine\nAB=2\n\n# end\n" {
		t.Errorf("got %q", got)
	}
}

func TestShellQuoteRoundTripsThroughSh(t *testing.T) {
	values := []string{"plain", "with space", `it's`, `"double"`, `$HOME \n back\slash`, "multi\nline", "", "'''"}
	var list []Value
	for i, v := range values {
		list = append(list, Value{Name: "V" + string(rune('A'+i)), Value: v})
	}
	script := Render(list)
	for i, v := range values {
		name := list[i].Name
		out, err := exec.Command("sh", "-c", script+`printf '%s' "$`+name+`"`).Output()
		if err != nil {
			t.Fatalf("sh: %v\n%s", err, script)
		}
		if string(out) != v {
			t.Errorf("%s: sh read %q, want %q", name, out, v)
		}
	}
}

func TestKindNeverShowsPlainValues(t *testing.T) {
	cases := map[string]string{
		"keychain:svc":       "Keychain svc",
		"op://Private/x/key": "1Password Private/x/key",
		"env:TOKEN":          "$TOKEN on the Mac",
		"cmd:pass show x":    "command: pass show x",
		"hunter2":            "plain value",
	}
	for ref, want := range cases {
		if got := Kind(ref); got != want {
			t.Errorf("Kind(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestResolve(t *testing.T) {
	var ran []string
	r := Resolver{
		Keychain: func(s string) (string, error) {
			if s == "good" {
				return "from-keychain", nil
			}
			return "", errors.New("missing")
		},
		LookupEnv: func(n string) (string, bool) {
			if n == "SET" {
				return "from-env", true
			}
			return "", false
		},
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			ran = append(ran, name+" "+strings.Join(args, " "))
			if name == "op" {
				return []byte("from-op"), nil
			}
			return []byte("from-cmd\n"), nil
		},
		HasProgram: func(string) bool { return true },
	}
	ctx := context.Background()
	for ref, want := range map[string]string{
		"keychain:good": "from-keychain",
		"env:SET":       "from-env",
		"op://v/item/f": "from-op",
		"cmd:echo from": "from-cmd",
		"just-a-value":  "just-a-value",
	} {
		got, err := r.Resolve(ctx, ref)
		if err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
	for _, bad := range []string{"keychain:nope", "env:UNSET"} {
		if _, err := r.Resolve(ctx, bad); err == nil {
			t.Errorf("Resolve(%q) succeeded, want an error", bad)
		}
	}
	if !strings.Contains(strings.Join(ran, "|"), "op read --no-newline op://v/item/f") {
		t.Errorf("op not called as expected: %v", ran)
	}

	r.HasProgram = func(string) bool { return false }
	if _, err := r.Resolve(ctx, "op://v/i/f"); err == nil || !strings.Contains(err.Error(), "isn't installed") {
		t.Errorf("missing op: err = %v", err)
	}
}

func TestResolveAllReportsFailuresAndKeepsTheRest(t *testing.T) {
	r := Resolver{
		Keychain:  func(string) (string, error) { return "", errors.New("missing") },
		LookupEnv: func(string) (string, bool) { return "ok", true },
	}
	values, errs := r.ResolveAll(context.Background(), []Secret{{"A", "env:A", ""}, {"B", "keychain:x", ""}})
	if len(values) != 1 || values[0].Name != "A" || len(errs) != 1 || errs["B"] == nil {
		t.Fatalf("values=%+v errs=%v", values, errs)
	}
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
