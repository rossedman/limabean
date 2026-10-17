// Package keychain stores generic passwords in the macOS login Keychain
// through the `security` command. Values are passed on stdin, never as
// arguments, so they don't appear in process listings.
//
// `security` reads a password from the terminal when it has one, prompting
// "password data for new item:" and ignoring stdin. So it always runs in a
// new session without a controlling terminal, where it reads stdin instead.
package keychain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
)

// ErrNotFound means there's no item for the service.
var ErrNotFound = errors.New("keychain item not found")

// Keychain reads and writes items for one account.
type Keychain struct {
	// Account is the item's account; it defaults to the current user name.
	Account string
	// Security is the path of the security command; "security" by default.
	Security string
}

// Default returns a Keychain for the current user.
func Default() Keychain {
	name := os.Getenv("USER")
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	return Keychain{Account: name, Security: "security"}
}

func (k Keychain) bin() string {
	if k.Security != "" {
		return k.Security
	}
	return "security"
}

// command runs security detached from the terminal (see the package doc).
func (k Keychain) command(args ...string) *exec.Cmd {
	cmd := exec.Command(k.bin(), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

// Get returns the item's value.
func (k Keychain) Get(service string) (string, error) {
	cmd := k.command("find-generic-password", "-a", k.Account, "-s", service, "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// Set creates or replaces the item. With -w as the last argument, security
// reads the value and its confirmation from stdin.
func (k Keychain) Set(service, value string) error {
	if strings.Contains(value, "\n") {
		return errors.New("Keychain values can't contain newlines")
	}
	cmd := k.command("add-generic-password", "-U", "-a", k.Account, "-s", service, "-w")
	cmd.Stdin = strings.NewReader(value + "\n" + value + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("security add-generic-password: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if got, err := k.Get(service); err != nil || got != value {
		return errors.New("the Keychain didn't keep the value")
	}
	return nil
}

// Delete removes the item. Deleting a missing item returns ErrNotFound.
func (k Keychain) Delete(service string) error {
	if err := k.command("delete-generic-password", "-a", k.Account, "-s", service).Run(); err != nil {
		return ErrNotFound
	}
	return nil
}
