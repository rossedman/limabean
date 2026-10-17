package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
)

func newCodeCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code [name] [repo]",
		Short: "Open a VM, or a repository in it, in VS Code or Cursor",
		Long: `Open a VM in your editor over SSH, using VS Code's Remote-SSH (or Cursor's
built-in equivalent). A stopped VM is started first.

With a repository, the editor opens that clone under ~/code. It's matched
against the VM's ghq clones the way "ghq list" matches: limabean,
rossedman/limabean and github.com/rossedman/limabean all find the same one.
If several match, you pick one. An absolute path in the VM works too.
Without one, the editor opens ~/code.

With a single argument that isn't a VM's name, it's taken as a repository
and you pick the VM.

The editor is the IDE setting (code by default), or --ide. Any editor whose
command line takes VS Code's --remote flag works: code, code-insiders,
cursor, windsurf. It connects to lima-<name> through ~/.ssh/config, so that
needs Lima's Include (limabean doctor checks it).`,
		Example: `  # dev's ~/code in VS Code
  limabean code dev

  # One repository
  limabean code dev limabean
  limabean code dev rossedman/limabean

  # In Cursor, this time
  limabean code dev limabean --ide cursor

  # Always use Cursor
  echo 'IDE="cursor"' >> ~/.config/limabean/config`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, repo, err := a.codeArgs(ctx, args)
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if inst, err = a.ensureRunning(ctx, inst); err != nil {
				return err
			}
			dir, err := a.codeDir(ctx, inst, repo)
			if err != nil {
				return err
			}
			home, _ := os.UserHomeDir()
			if inc, _ := sshInclude(filepath.Join(home, ".ssh", "config")); inc == 0 {
				a.UI.Warn("~/.ssh/config doesn't include Lima's host entries, so the editor may not find lima-%s", name)
				a.UI.Hint("limabean init   (or see: limabean doctor)")
			}
			ide := a.Cfg.IDE
			if ide == "" {
				ide = "code"
			}
			a.UI.Info("Opening %s in %s", a.UI.Code(name+":"+dir), ide)
			return a.openEditor(ctx, ide, "--remote", "ssh-remote+lima-"+name, dir)
		},
	}
	cmd.Flags().String("ide", "", "editor command to open, e.g. code or cursor (default: the IDE setting, or code)")
	_ = a.V.BindPFlag(config.KeyIDE, cmd.Flags().Lookup("ide"))
	return cmd
}

// codeArgs works out the VM and repository from code's arguments. A single
// argument is the VM if one has that name, and otherwise a repository.
func (a *App) codeArgs(ctx context.Context, args []string) (name, repo string, err error) {
	switch len(args) {
	case 2:
		return args[0], args[1], nil
	case 1:
		if _, ok, err := a.Lima.Get(ctx, args[0]); err != nil || ok {
			return args[0], "", err
		}
		name, err := a.vmArg(ctx, nil, "open "+args[0]+" from")
		return name, args[0], err
	}
	name, err = a.vmArg(ctx, nil, "open")
	return name, "", err
}

// codeDir returns the directory in the VM to open: the code directory, an
// absolute path, or the ghq clone matching repo.
func (a *App) codeDir(ctx context.Context, inst lima.Instance, repo string) (string, error) {
	if strings.HasPrefix(repo, "/") {
		return repo, nil
	}
	if repo == "" {
		out, err := a.Lima.Run(ctx, inst, nil, `d=`+homeQuote(a.Cfg.CodeDir)+` && mkdir -p "$d" && cd "$d" && pwd`)
		if err != nil {
			return "", fmt.Errorf("finding %s in %s: %w", a.Cfg.CodeDir, inst.Name, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	q := repoQuery(repo)
	out, err := a.Lima.Run(ctx, inst, nil, "ghq list -p "+secrets.ShellQuote(q))
	if err != nil {
		return "", fmt.Errorf("listing repositories in %s: %w", inst.Name, err)
	}
	matches := repoMatches(strings.Fields(string(out)), q)
	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) == 0:
		return "", fmt.Errorf("no repository matching %q in %s; clone it with: limabean get %s %s", repo, inst.Name, inst.Name, repo)
	case !a.UI.Prompting:
		return "", fmt.Errorf("%q matches several repositories in %s; name one:\n  %s", repo, inst.Name, strings.Join(matches, "\n  "))
	}
	return a.UI.Select("Repository to open", matches)
}

// repoQuery turns a repository as it might be typed (a URL, git@host:path,
// owner/repo) into the host/owner/repo form ghq lays clones out by.
func repoQuery(repo string) string {
	q := repo
	if _, rest, ok := strings.Cut(q, "://"); ok {
		q = rest
		if _, host, ok := strings.Cut(q, "@"); ok { // ssh://git@host/...
			q = host
		}
	} else if user, rest, ok := strings.Cut(q, "@"); ok && !strings.Contains(user, "/") {
		q = strings.Replace(rest, ":", "/", 1) // git@host:owner/repo
	}
	return strings.TrimSuffix(strings.TrimSuffix(q, "/"), ".git")
}

// repoMatches narrows ghq's matches for q to the clones it names in full
// (limabean finds .../rossedman/limabean but not .../limabean-docs), or
// keeps them all when none does.
func repoMatches(paths []string, q string) []string {
	var exact []string
	for _, p := range paths {
		if strings.HasSuffix(p, "/"+q) {
			exact = append(exact, p)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return paths
}

// homeQuote quotes a path for the VM's shell, leaving a leading ~ for the
// VM's home directory.
func homeQuote(p string) string {
	switch {
	case p == "~":
		return `"$HOME"`
	case strings.HasPrefix(p, "~/"):
		return `"$HOME"/` + secrets.ShellQuote(p[2:])
	}
	return secrets.ShellQuote(p)
}

// openEditor runs the editor's command line tool, which hands the window to
// the running app and returns.
func openEditor(ctx context.Context, ide string, args ...string) error {
	path, err := lookPath(ide)
	if err != nil {
		hint := ""
		switch filepath.Base(ide) {
		case "code", "code-insiders":
			hint = `; in VS Code, run "Shell Command: Install 'code' command in PATH" from the Command Palette`
		case "cursor":
			hint = `; in Cursor, run "Shell Command: Install 'cursor' command in PATH" from the Command Palette`
		}
		return fmt.Errorf("%s isn't on your PATH%s", ide, hint)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
