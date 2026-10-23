package cli

import (
	"context"
	"crypto/sha1" //nolint:gosec // macOS names certificates in trust settings by SHA-1
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A company proxy that inspects HTTPS (Zscaler and the like) re-signs
// what it inspects with its own root certificate. The Mac trusts that root
// because an admin added it; a VM doesn't, so downloads in it fail with
// "certificate verify failed" or "UnknownIssuer". limabean hands Lima the
// roots an admin told the Mac to trust without conditions, and Lima's
// caCerts installs them in the VM before anything is provisioned.

// systemKeychain holds the certificates an admin or MDM installed.
const systemKeychain = "/Library/Keychains/System.keychain"

// runSecurity runs macOS's security command.
func runSecurity(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "/usr/bin/security", args...).Output()
}

// macRootCAs returns, as PEM, the root certificates an admin told this Mac
// to trust for everything: the admin trust settings' entries with no
// conditions, found in the System keychain. Expired certificates and ones
// that aren't CAs are left out.
func (a *App) macRootCAs(ctx context.Context) ([]string, error) {
	dir, err := os.MkdirTemp("", "limabean-trust-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	plist := filepath.Join(dir, "trust.plist")
	if _, err := a.security(ctx, "trust-settings-export", "-d", plist); err != nil {
		// No admin trust settings at all is the common case, and not an error.
		return nil, nil
	}
	b, err := os.ReadFile(plist)
	if err != nil {
		return nil, err
	}
	trusted, err := trustedHashes(b)
	if err != nil || len(trusted) == 0 {
		return nil, err
	}
	pems, err := a.security(ctx, "find-certificate", "-a", "-p", systemKeychain)
	if err != nil {
		return nil, err
	}
	var out []string
	for rest := pems; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		sum := sha1.Sum(block.Bytes) //nolint:gosec // only to look the certificate up
		if !trusted[strings.ToUpper(hex.EncodeToString(sum[:]))] {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.IsCA || time.Now().After(cert.NotAfter) {
			continue
		}
		p := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: block.Bytes}))
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out, nil
}

// caCertSets returns the --set expression that has Lima install the Mac's
// admin-trusted roots in a VM, or nothing when there are none or CA_CERTS
// is off. Failing to read them is a warning: the VM still works where no
// proxy is in the way.
func (a *App) caCertSets(ctx context.Context) []string {
	if !a.Cfg.CACerts {
		return nil
	}
	certs, err := a.macRootCAs(ctx)
	if err != nil {
		a.UI.Warn("Couldn't read the Mac's trusted root certificates: %v", err)
		return nil
	}
	if len(certs) == 0 {
		return nil
	}
	b, _ := json.Marshal(certs)
	return []string{".caCerts.certs += " + string(b)}
}

// trustedHashes reads `security trust-settings-export` output: a plist
// whose trustList maps each certificate's SHA-1 to its trust settings. A
// certificate with no trustSettings, or an empty list of them, is trusted
// as a root for everything; one with settings is trusted only for some
// uses, or distrusted, and is left out.
func trustedHashes(b []byte) (map[string]bool, error) {
	v, err := decodePlist(b)
	if err != nil {
		return nil, err
	}
	top, _ := v.(map[string]any)
	list, _ := top["trustList"].(map[string]any)
	out := map[string]bool{}
	for hash, entry := range list {
		e, _ := entry.(map[string]any)
		if settings, ok := e["trustSettings"]; ok {
			if s, _ := settings.([]any); len(s) > 0 {
				continue
			}
		}
		out[strings.ToUpper(hash)] = true
	}
	return out, nil
}

// decodePlist decodes an XML property list into maps, slices and strings
// (numbers, dates and data stay as their text).
func decodePlist(b []byte) (any, error) {
	d := xml.NewDecoder(strings.NewReader(string(b)))
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local != "plist" {
			return plistValue(d, se)
		}
	}
}

func plistValue(d *xml.Decoder, se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "dict":
		m := map[string]any{}
		key := ""
		for {
			tok, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "key" {
					var k string
					if err := d.DecodeElement(&k, &t); err != nil {
						return nil, err
					}
					key = k
					continue
				}
				v, err := plistValue(d, t)
				if err != nil {
					return nil, err
				}
				m[key] = v
			case xml.EndElement:
				return m, nil
			}
		}
	case "array":
		var a []any
		for {
			tok, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				v, err := plistValue(d, t)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			case xml.EndElement:
				return a, nil
			}
		}
	case "true", "false":
		return se.Name.Local == "true", d.Skip()
	default:
		var s string
		if err := d.DecodeElement(&s, &se); err != nil {
			return nil, err
		}
		return strings.TrimSpace(s), nil
	}
}
