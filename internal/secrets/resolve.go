package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Resolver turns references into values. Its fields are the outside world,
// so tests can replace them.
type Resolver struct {
	// Keychain returns a Keychain item's value.
	Keychain func(service string) (string, error)
	// LookupEnv reads the environment limabean runs in.
	LookupEnv func(name string) (string, bool)
	// Run runs a program and returns its standard output.
	Run func(ctx context.Context, name string, args ...string) ([]byte, error)
	// HasProgram reports whether a program is on PATH.
	HasProgram func(name string) bool
}

// Resolve returns the value a reference points at.
func (r Resolver) Resolve(ctx context.Context, ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "keychain:"):
		service := strings.TrimPrefix(ref, "keychain:")
		v, err := r.Keychain(service)
		if err != nil {
			return "", fmt.Errorf("no Keychain item %q", service)
		}
		return v, nil
	case strings.HasPrefix(ref, "op://"):
		if !r.HasProgram("op") {
			return "", errors.New("the 1Password CLI isn't installed (brew install 1password-cli)")
		}
		out, err := r.Run(ctx, "op", "read", "--no-newline", ref)
		if err != nil {
			return "", errors.New("op couldn't read it (is 1Password unlocked and the CLI signed in?)")
		}
		return string(out), nil
	case strings.HasPrefix(ref, "env:"):
		name := strings.TrimPrefix(ref, "env:")
		v, ok := r.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("$%s isn't set where limabean runs", name)
		}
		return v, nil
	case strings.HasPrefix(ref, "cmd:"):
		out, err := r.Run(ctx, "sh", "-c", strings.TrimPrefix(ref, "cmd:"))
		if err != nil {
			return "", fmt.Errorf("the command failed: %v", err)
		}
		return strings.TrimRight(string(out), "\n"), nil
	default:
		return ref, nil
	}
}

// ResolveAll resolves every secret. Ones that fail are reported in errs,
// keyed by name, and left out of the values.
func (r Resolver) ResolveAll(ctx context.Context, list []Secret) (values []Value, errs map[string]error) {
	errs = map[string]error{}
	for _, s := range list {
		v, err := r.Resolve(ctx, s.Ref)
		if err != nil {
			errs[s.Name] = err
			continue
		}
		values = append(values, Value{Name: s.Name, Value: v})
	}
	return values, errs
}

// RunCommand runs a program with no stdin and returns its stdout; on failure
// the error includes stderr. It's the default Resolver.Run.
func RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return out, fmt.Errorf("%w: %s", err, msg)
		}
	}
	return out, err
}
