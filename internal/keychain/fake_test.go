package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSecurity is a stand-in for /usr/bin/security that keeps items as files
// and logs its arguments, so tests can check values never reach argv.
const fakeSecurity = `#!/bin/sh
echo "$@" >> "$FAKE_KC/argv.log"
cmd=$1; shift
svc=""
while [ $# -gt 0 ]; do
  case $1 in -s) svc=$2; shift ;; -a) shift ;; esac
  shift
done
case $cmd in
  find-generic-password) [ -f "$FAKE_KC/$svc" ] || exit 44; cat "$FAKE_KC/$svc"; echo ;;
  add-generic-password)
    read -r v; read -r v2
    [ "$v" = "$v2" ] || { echo "values differ" >&2; exit 1; }
    [ -n "$FAKE_KC_FAIL" ] && { echo "write refused" >&2; exit 1; }
    if [ -n "$FAKE_KC_MANGLE" ]; then printf 'mangled' > "$FAKE_KC/$svc"; else printf '%s' "$v" > "$FAKE_KC/$svc"; fi ;;
  delete-generic-password) [ -f "$FAKE_KC/$svc" ] || exit 44; rm "$FAKE_KC/$svc" ;;
esac
`

func fake(t *testing.T) (Keychain, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "security")
	if err := os.WriteFile(bin, []byte(fakeSecurity), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_KC", dir)
	return Keychain{Account: "tester", Security: bin}, dir
}

func TestFakeRoundTripKeepsValuesOutOfArgv(t *testing.T) {
	k, dir := fake(t)
	value := `a b "c" 'd' $e`
	if err := k.Set("svc", value); err != nil {
		t.Fatal(err)
	}
	got, err := k.Get("svc")
	if err != nil || got != value {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := k.Delete("svc"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get("svc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete: %v", err)
	}
	if err := k.Delete("svc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing: %v", err)
	}
	argv, _ := os.ReadFile(filepath.Join(dir, "argv.log"))
	if strings.Contains(string(argv), "$e") || strings.Contains(string(argv), `"c"`) {
		t.Fatalf("value leaked into argv:\n%s", argv)
	}
	if !strings.Contains(string(argv), "-a tester -s svc") {
		t.Fatalf("account/service not passed:\n%s", argv)
	}
}

func TestSetFailures(t *testing.T) {
	k, _ := fake(t)
	t.Setenv("FAKE_KC_FAIL", "1")
	if err := k.Set("svc", "v"); err == nil || !strings.Contains(err.Error(), "write refused") {
		t.Fatalf("refused write: err = %v", err)
	}
	t.Setenv("FAKE_KC_FAIL", "")
	t.Setenv("FAKE_KC_MANGLE", "1")
	if err := k.Set("svc", "v"); err == nil || !strings.Contains(err.Error(), "didn't keep") {
		t.Fatalf("mangled value: err = %v", err)
	}
}

func TestDefaultsAndBin(t *testing.T) {
	d := Default()
	if d.Account == "" || d.Security != "security" {
		t.Fatalf("Default = %+v", d)
	}
	if (Keychain{}).bin() != "security" {
		t.Fatal("empty Security should mean the security command")
	}
}

// Regression: in a terminal, security prompted "password data for new item"
// instead of reading the piped value. Every call must run without one.
func TestSecurityRunsWithoutATerminal(t *testing.T) {
	cmd := (Keychain{Security: "security"}).command("find-generic-password")
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("security must run in a new session, detached from the terminal")
	}
}
