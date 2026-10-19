package cli

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // as macOS names them
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/rossedman/limabean/internal/ui"
)

// testCert makes a certificate, as PEM, and its SHA-1 as macOS writes it.
func testCert(t *testing.T, name string, ca bool, notAfter time.Time) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
		IsCA: ca, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(der) //nolint:gosec
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), strings.ToUpper(hex.EncodeToString(sum[:]))
}

// trustPlist is `security trust-settings-export` output: always is
// trusted for everything, limited only for some uses.
func trustPlist(always []string, limited ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>trustList</key><dict>`)
	for _, h := range always {
		fmt.Fprintf(&b, `<key>%s</key><dict><key>issuerName</key><data>MIGh</data><key>modDate</key><date>2024-03-20T16:42:37Z</date><key>trustSettings</key><array/></dict>`, h)
	}
	for _, h := range limited {
		fmt.Fprintf(&b, `<key>%s</key><dict><key>trustSettings</key><array><dict><key>kSecTrustSettingsResult</key><integer>3</integer><key>kSecTrustSettingsAllowedError</key><true/></dict></array></dict>`, h)
	}
	b.WriteString(`</dict><key>trustVersion</key><integer>1</integer></dict></plist>`)
	return b.String()
}

func TestNewTrustsTheMacsAdminRoots(t *testing.T) {
	year := time.Now().AddDate(1, 0, 0)
	proxy, proxyHash := testCert(t, "Zscaler Root CA", true, year)
	denied, deniedHash := testCert(t, "Denied CA", true, year)
	leaf, leafHash := testCert(t, "a leaf", false, year)
	old, oldHash := testCert(t, "Expired CA", true, time.Now().Add(-time.Minute))
	other, _ := testCert(t, "Not in the trust settings", true, year)

	h := newHarness(t)
	h.world.trustPlist = trustPlist([]string{proxyHash, leafHash, oldHash}, deniedHash)
	h.world.keychainPEM = proxy + denied + leaf + old + other + proxy // a duplicate too
	h.must("", "new", "dev")
	create := findCall(h.world, "limactl start --name=dev")
	if !strings.Contains(create, ".caCerts.certs += [") || strings.Count(create, "BEGIN CERTIFICATE") != 1 {
		t.Fatalf("create should trust only the proxy's root:\n%s", create)
	}
	if !strings.Contains(create, strings.SplitN(proxy, "\n", 3)[1]) {
		t.Errorf("the proxy's root isn't the one passed:\n%s", create)
	}

	// CA_CERTS=no leaves the VM with Ubuntu's own list, and asks nothing.
	h2 := newHarness(t)
	h2.world.trustPlist = h.world.trustPlist
	h2.world.keychainPEM = proxy
	t.Setenv("LIMABEAN_CA_CERTS", "no")
	h2.must("", "new", "dev")
	if strings.Contains(findCall(h2.world, "limactl start --name=dev"), "caCerts") || h2.world.called("security") {
		t.Errorf("CA_CERTS=no still passed certificates: %v", h2.world.calls)
	}
}

func TestNewWithoutAdminTrustSettings(t *testing.T) {
	h := newHarness(t)
	out := h.must("", "new", "dev")
	if strings.Contains(findCall(h.world, "limactl start --name=dev"), "caCerts") || strings.Contains(out, "trusted root") {
		t.Errorf("no trust settings should mean no certificates and no warning:\n%s", out)
	}
}

func TestCACertsReadFailureIsAWarning(t *testing.T) {
	_, hash := testCert(t, "x", true, time.Now().AddDate(1, 0, 0))
	h := newHarness(t)
	h.world.trustPlist = trustPlist([]string{hash})
	a := h.app(ui.NewPlain(&bytes.Buffer{}))
	a.security = func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "find-certificate" {
			return nil, errors.New("keychain locked")
		}
		return nil, nil // writes no file: reading it fails
	}
	if _, err := a.macRootCAs(t.Context()); err == nil {
		t.Error("an unreadable export should be an error")
	}

	// Creating a VM goes on without them, and says why.
	h.world.keychainFails = true
	out := h.must("", "new", "dev")
	if !strings.Contains(out, "Couldn't read the Mac's trusted root certificates: keychain locked") ||
		strings.Contains(findCall(h.world, "limactl start --name=dev"), "caCerts") {
		t.Errorf("a keychain failure should warn and pass nothing:\n%s", out)
	}
}

func TestTrustedHashes(t *testing.T) {
	got, err := trustedHashes([]byte(trustPlist([]string{"aa"}, "BB")))
	if err != nil || !got["AA"] || got["BB"] || len(got) != 1 {
		t.Errorf("trustedHashes = %v, %v", got, err)
	}
	// An entry without trustSettings at all, as macOS writes for most roots.
	got, err = trustedHashes([]byte(`<plist><dict><key>trustList</key><dict><key>CC</key><dict><key>modDate</key><date>x</date></dict></dict></dict></plist>`))
	if err != nil || !got["CC"] {
		t.Errorf("no trustSettings: %v, %v", got, err)
	}
	if _, err := trustedHashes([]byte("not a plist")); err == nil {
		t.Error("garbage: no error")
	}
	if _, err := trustedHashes([]byte("<plist><dict><key>x</key><array><string>y")); err == nil {
		t.Error("truncated: no error")
	}
	if _, err := trustedHashes([]byte("<plist><dict><key>x</key><dict><key>y")); err == nil {
		t.Error("truncated dict: no error")
	}
}

// findCall returns the first recorded call that starts with prefix.
func findCall(w *fakeWorld, prefix string) string {
	for _, c := range w.calls {
		if strings.HasPrefix(c, prefix) {
			return c
		}
	}
	return ""
}
