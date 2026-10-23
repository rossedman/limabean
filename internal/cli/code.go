package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
)

// newCodeCmd builds `limabean code`, which opens a VM, or a repository in
// it, in an editor.
func newCodeCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code [name] [repo]",
		Short: "Open a VM, or a repository in it, in VS Code, Cursor or another editor",
		Long: `Open a VM in your editor over SSH, using its Remote - SSH extension. A stopped VM is started first.

With a repository, the editor opens that clone under ~/code. It's matched
against the VM's ghq clones the way "ghq list" matches: limabean,
rossedman/limabean and github.com/rossedman/limabean all find the same one.
If several match, you pick one. An absolute path in the VM works too.
Without one, the editor opens ~/code.

With a single argument that isn't a VM's name, it's taken as a repository
and you pick the VM.

The editor is the IDE setting, or --ide: code, code-insiders, cursor,
windsurf, codium, or any other editor whose command line takes VS Code's
--remote flag. Without one, or when the one named isn't installed,
limabean opens the first of those it finds. It finds an editor on your
PATH or inside its app in /Applications, so "Install 'code' command in
PATH" isn't needed. It connects to lima-<name> through ~/.ssh/config, so
that needs Lima's Include (limabean doctor checks it).

VS Code, VS Code Insiders and VSCodium need a Remote - SSH extension to
open a VM, and each keeps its own extensions. If it's missing, limabean
offers to install it: ms-vscode-remote.remote-ssh, or
jeanp413.open-remote-ssh for VSCodium. Cursor and Windsurf ship their own,
so limabean doesn't check them.`,
		Example: `  # dev's ~/code in VS Code
  limabean code dev

  # One repository
  limabean code dev limabean
  limabean code dev rossedman/limabean

  # In Cursor, or VS Code Insiders, this time
  limabean code dev limabean --ide cursor
  limabean code dev --ide code-insiders

  # Always use VS Code Insiders
  echo 'IDE="code-insiders"' >> ~/.config/limabean/config`,
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
			ide, path, err := pickEditor(a.Cfg.IDE)
			if err != nil {
				return err
			}
			if a.Cfg.IDE != "" && ide != a.Cfg.IDE {
				a.UI.Warn("%s isn't installed, so this opens %s", a.Cfg.IDE, ide)
				a.UI.Hint(fmt.Sprintf("To make it the default: IDE=%q in ~/.config/limabean/config", ide))
			}
			if err := a.ensureRemoteSSH(ctx, ide, path); err != nil {
				return err
			}
			a.UI.Info("Opening %s in %s", a.UI.Code(name+":"+dir), ide)
			return a.openEditor(ctx, path, "--remote", "ssh-remote+lima-"+name, dir)
		},
	}
	cmd.Flags().String("ide", "", "editor to open, e.g. code-insiders or cursor (default: the IDE setting, or the first one installed)")
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
		name, err := a.projectVMArg(ctx, nil, "open "+args[0]+" from")
		return name, args[0], err
	}
	name, err = a.projectVMArg(ctx, nil, "open")
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

// editors take VS Code's --remote flag; limabean looks for them in this
// order when none is named, or the one named isn't installed.
var editors = []string{"code", "code-insiders", "cursor", "windsurf", "codium"}

// editorApps is where each editor's command line tool sits inside its app,
// for when "Install ... command in PATH" was never run.
var editorApps = map[string]string{
	"code":          "Visual Studio Code.app/Contents/Resources/app/bin/code",
	"code-insiders": "Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code-insiders",
	"cursor":        "Cursor.app/Contents/Resources/app/bin/cursor",
	"windsurf":      "Windsurf.app/Contents/Resources/app/bin/windsurf",
	"codium":        "VSCodium.app/Contents/Resources/app/bin/codium",
}

// applicationDirs are where macOS apps are installed; tests replace it.
var applicationDirs = func() []string {
	home, _ := os.UserHomeDir()
	return []string{"/Applications", filepath.Join(home, "Applications")}
}

// findEditor returns the command for an editor: on PATH, or inside its app.
func findEditor(ide string) (string, bool) {
	if path, err := lookPath(ide); err == nil {
		return path, true
	}
	if app, ok := editorApps[ide]; ok {
		for _, dir := range applicationDirs() {
			if path := filepath.Join(dir, app); fileExists(path) {
				return path, true
			}
		}
	}
	return "", false
}

// pickEditor returns the editor to open and its command: the one named if
// it's installed, else the first of editors that is.
func pickEditor(named string) (ide, path string, err error) {
	if named != "" {
		if path, ok := findEditor(named); ok {
			return named, path, nil
		}
	}
	for _, ide := range editors {
		if path, ok := findEditor(ide); ok {
			return ide, path, nil
		}
	}
	if named != "" && !slices.Contains(editors, named) {
		return "", "", fmt.Errorf("%s isn't on your PATH, and none of %s is installed", named, strings.Join(editors, ", "))
	}
	return "", "", fmt.Errorf("no editor found: limabean code opens %s, on your PATH or in /Applications", strings.Join(editors, ", "))
}

// remoteSSHExtensions give an editor its ssh-remote+ paths, for the editors
// that don't have one built in. Without it, the editor opens a window that
// says "Unable to resolve resource vscode-remote://…".
var remoteSSHExtensions = map[string]string{
	"code":          "ms-vscode-remote.remote-ssh",
	"code-insiders": "ms-vscode-remote.remote-ssh",
	"codium":        "jeanp413.open-remote-ssh",
}

// ensureRemoteSSH installs the editor's Remote - SSH extension if it's
// missing, after asking. If the editor can't list its extensions, it opens
// the VM anyway.
func (a *App) ensureRemoteSSH(ctx context.Context, ide, path string) error {
	ext, ok := remoteSSHExtensions[ide]
	if !ok {
		return nil
	}
	out, err := a.editorOutput(ctx, path, "--list-extensions")
	if err != nil {
		return nil
	}
	for line := range strings.Lines(string(out)) {
		if strings.EqualFold(strings.TrimSpace(line), ext) {
			return nil
		}
	}
	install := ide + " --install-extension " + ext
	ok, err = a.UI.Confirm(fmt.Sprintf("%s needs the Remote - SSH extension (%s) to open a VM. Install it?", ide, ext), true)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s can't open a VM without Remote - SSH: %s", ide, install)
	}
	a.UI.Info("Installing %s in %s", ext, ide)
	if out, err := a.editorOutput(ctx, path, "--install-extension", ext); err != nil {
		return fmt.Errorf("%s failed: %w\n%s", install, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// editorOutput runs the editor's command line tool and returns its output.
func editorOutput(ctx context.Context, path string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, path, args...).CombinedOutput()
}

// openEditor runs the editor's command line tool, which hands the window to
// the running app and returns.
func openEditor(ctx context.Context, path string, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
