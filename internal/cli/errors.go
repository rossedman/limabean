package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Exit codes are part of limabean's interface: scripts branch on them, so
// they're documented in docs/cli.md and never renumbered. run exits with
// its command's own status instead.
const (
	exitOK       = 0
	exitError    = 1 // anything else that went wrong
	exitUsage    = 2 // a bad flag or argument, or a missing value without a terminal
	exitAuth     = 3 // reserved: authentication or permission
	exitNotFound = 4 // the VM, project file, recipe or secret doesn't exist
	exitConflict = 5 // it exists already, or a check refused (unsaved work, a file in the way)
)

// cliError is an error with an exit code, and a stable name for --json.
type cliError struct {
	code   int
	kind   string
	err    error
	silent bool // already explained to the user
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErr(format string, a ...any) error {
	return &cliError{code: exitUsage, kind: "usage", err: fmt.Errorf(format, a...)}
}

func notFoundErr(format string, a ...any) error {
	return &cliError{code: exitNotFound, kind: "not_found", err: fmt.Errorf(format, a...)}
}

func conflictErr(format string, a ...any) error {
	return &cliError{code: exitConflict, kind: "conflict", err: fmt.Errorf(format, a...)}
}

// errUnsaved stops a command whose unsaved-work check already listed what
// it found.
var errUnsaved = &cliError{code: exitConflict, kind: "conflict", err: errors.New("unsaved work"), silent: true}

// classify gives an error its exit code and kind. Cobra's own parsing
// errors are usage errors.
func classify(err error) (int, string) {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code, ce.kind
	}
	msg := err.Error()
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand flag", "flag needs an argument", "invalid argument", "required flag", "accepts ", "requires at least", "requires at most", "if any flags in the group"} {
		if strings.HasPrefix(msg, p) || strings.Contains(msg, " "+p) {
			return exitUsage, "usage"
		}
	}
	return exitError, "error"
}

// usageArgs marks every command's argument errors as usage errors.
func usageArgs(c *cobra.Command) {
	if args := c.Args; args != nil {
		c.Args = func(cmd *cobra.Command, a []string) error {
			if err := args(cmd, a); err != nil {
				return &cliError{code: exitUsage, kind: "usage", err: err}
			}
			return nil
		}
	}
	for _, sub := range c.Commands() {
		usageArgs(sub)
	}
}

// wantsJSON reports whether a command line asked for --json, before any --.
func wantsJSON(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--json" {
			return true
		}
	}
	return false
}

// writeJSONError prints an error as JSON, for --json callers.
func writeJSONError(w io.Writer, err error) {
	code, kind := classify(err)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
		"code": kind, "exit": code, "message": err.Error(),
	}})
}

// bugReport is what a crash prints: it's limabean's fault, not the user's,
// and here's how to report it.
func bugReport(w io.Writer, version string, args []string, r any) {
	fmt.Fprintf(w, "limabean crashed. That's a bug in limabean, not something you did.\n")
	fmt.Fprintf(w, "Please report it at https://github.com/rossedman/limabean/issues/new with:\n\n")
	fmt.Fprintf(w, "  version: %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "  command: limabean %s\n", strings.Join(args, " "))
	fmt.Fprintf(w, "  error:   %v\n\n%s", r, debug.Stack())
}
