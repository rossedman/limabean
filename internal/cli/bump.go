package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/pins"
	"github.com/rossedman/limabean/internal/secrets"
)

func newBumpCmd(a *App) *cobra.Command {
	var vm string
	var major, dryRun bool
	cmd := &cobra.Command{
		Use:   "bump [file...]",
		Short: "Update the pinned tool versions in mise TOML files",
		Long: `Look up the newest version of every tool pinned in mise TOML files and
rewrite the pins, keeping comments and layout. Without files it updates the
packs, templates/packs/*.toml, and any templates/mise/*.toml, under the
current directory: a limabean checkout, or your own templates folder.

Each pin stays within its major version (its minor below 1.0, where minors
can break, and its vendor, like temurin- for Java) unless you pass --major.
Newer majors are listed either way.

Versions come from "mise latest", run on your Mac if mise is installed
(brew install mise) or in a running VM with --vm, since every VM has mise.
VMs pick up new pins when they're rebuilt.`,
		Example: `  # In a limabean checkout
  limabean bump

  # See what would change
  limabean bump --dry-run

  # Without mise on the Mac
  limabean bump --vm dev

  # One pack, majors included
  limabean bump templates/packs/node.toml --major`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			files := args
			if len(files) == 0 {
				packs, _ := filepath.Glob(filepath.Join("templates", "packs", "*.toml"))
				envs, _ := filepath.Glob(filepath.Join("templates", "mise", "*.toml"))
				files = append(packs, envs...)
				if len(files) == 0 {
					return errors.New("no templates/packs/*.toml here; run it in a limabean checkout or name the files")
				}
			}
			latest, err := a.latestLookup(ctx, vm)
			if err != nil {
				return err
			}
			failed := false
			for _, f := range files {
				ok, err := a.bumpFile(ctx, f, latest, major, dryRun)
				if err != nil {
					return err
				}
				failed = failed || !ok
			}
			if failed {
				return errSilent
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vm, "vm", "", "look versions up with mise in this running VM")
	cmd.Flags().BoolVar(&major, "major", false, "also move pins to new major versions")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without writing")
	return cmd
}

// latestLookup returns how to ask mise for a tool's newest version: on the
// Mac, or in a VM.
func (a *App) latestLookup(ctx context.Context, vm string) (func(context.Context, string) (string, error), error) {
	if vm == "" {
		if _, err := lookPath("mise"); err != nil {
			return nil, errors.New("mise isn't installed here (brew install mise), or use --vm <name> to ask a VM")
		}
		return a.miseLatest, nil
	}
	inst, err := a.vm(ctx, vm)
	if err != nil {
		return nil, err
	}
	if !inst.Running() {
		return nil, fmt.Errorf("%s is stopped (limabean start %s)", vm, vm)
	}
	return func(ctx context.Context, spec string) (string, error) {
		out, err := a.Lima.Run(ctx, inst, nil, "mise latest "+secrets.ShellQuote(spec))
		return strings.TrimSpace(string(out)), err
	}, nil
}

// miseLatest runs `mise latest <spec>` on the Mac.
func miseLatest(ctx context.Context, spec string) (string, error) {
	out, err := exec.CommandContext(ctx, "mise", "latest", spec).Output()
	return strings.TrimSpace(string(out)), err
}

// bumpFile updates one file's pins and reports whether every lookup worked.
func (a *App) bumpFile(ctx context.Context, path string, latest func(context.Context, string) (string, error), major, dryRun bool) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	src := string(b)
	list := pins.Parse(src)
	if len(list) == 0 {
		a.UI.Info("%s: no pinned tools", path)
		return true, nil
	}

	type found struct{ inSeries, newest string }
	results := make([]found, len(list))
	errs := make([]error, len(list))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	err = a.UI.Spin(ctx, fmt.Sprintf("Looking up %d tools in %s", len(list), path), func(ctx contextT) error {
		for i, p := range list {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				// A vendor-prefixed version (temurin-25…) stays with its vendor.
				spec := p.Tool
				if prefix := pins.Prefix(p.Version); prefix != "" {
					spec += "@" + prefix
				}
				newest, err := latest(ctx, spec)
				if err != nil {
					errs[i] = err
					return
				}
				results[i].newest = newest
				if s := pins.Series(p.Version); s != "" && pins.Series(newest) != s {
					results[i].inSeries, errs[i] = latest(ctx, p.Tool+"@"+s)
				} else {
					results[i].inSeries = newest
				}
			}()
		}
		wg.Wait()
		return nil
	})
	if err != nil {
		return false, err
	}

	changes := map[string]string{}
	var rows [][]string
	ok := true
	for i, p := range list {
		r := results[i]
		target := r.inSeries
		if major {
			target = r.newest
		}
		note := ""
		switch {
		case errs[i] != nil:
			note = "lookup failed: " + firstLine(errs[i])
			ok = false
		case !major && pins.Newer(r.newest, r.inSeries) && pins.Newer(r.newest, p.Version):
			note = r.newest + " available (--major)"
		}
		if errs[i] == nil && pins.Newer(target, p.Version) {
			changes[p.Tool] = target
		} else {
			target = ""
		}
		if target != "" || note != "" {
			rows = append(rows, []string{p.Tool, p.Version, firstNonEmpty(target, "-"), note})
		}
	}
	if len(rows) > 0 {
		a.UI.Table([]string{"TOOL", "PINNED", "NEW", "NOTE"}, rows)
	}
	switch {
	case len(changes) == 0:
		a.UI.OK("%s: all %d pins are up to date", path, len(list))
	case dryRun:
		a.UI.Info("%s: %d pin(s) would change (dry run)", path, len(changes))
	default:
		if err := os.WriteFile(path, []byte(pins.Rewrite(src, changes)), 0o644); err != nil {
			return false, err
		}
		a.UI.OK("%s: updated %d pin(s)", path, len(changes))
		a.UI.Hint("VMs get them when rebuilt: limabean rebuild <name>")
	}
	return ok, nil
}

// firstLine is an error's first line, for a table cell.
func firstLine(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		err = errors.New(string(exit.Stderr))
	}
	s, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
	return s
}
