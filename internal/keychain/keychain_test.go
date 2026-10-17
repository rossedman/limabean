package keychain

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

// The real Keychain is only touched with LIMABEAN_TEST_KEYCHAIN=1, under a
// throwaway service name that the test deletes.
func TestRoundTrip(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("LIMABEAN_TEST_KEYCHAIN") != "1" {
		t.Skip("set LIMABEAN_TEST_KEYCHAIN=1 on macOS to test against the login Keychain")
	}
	k := Default()
	service := fmt.Sprintf("limabean-test-%d", os.Getpid())
	t.Cleanup(func() { _ = k.Delete(service) })

	value := `a b "c" 'd' $e \f`
	if err := k.Set(service, value); err != nil {
		t.Fatal(err)
	}
	got, err := k.Get(service)
	if err != nil || got != value {
		t.Fatalf("Get = %q, %v; want %q", got, err, value)
	}
	if err := k.Set(service, "updated"); err != nil {
		t.Fatal(err)
	}
	if got, _ := k.Get(service); got != "updated" {
		t.Fatalf("after update Get = %q", got)
	}
	if err := k.Delete(service); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(service); err != ErrNotFound {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
	if err := k.Delete(service); err != ErrNotFound {
		t.Fatalf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestSetRejectsNewlines(t *testing.T) {
	if err := (Keychain{Account: "x"}).Set("svc", "a\nb"); err == nil {
		t.Fatal("want an error for a multi-line value")
	}
}
