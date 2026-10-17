package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/claude"
	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
	"github.com/rossedman/limabean/internal/templates"
)

var lookPath = exec.LookPath

var vmNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)

// validVMName checks a name works as a Lima instance and an SSH host alias.
func validVMName(name string) error {
	if !vmNameRE.MatchString(name) {
		return errors.New("use letters, digits and dashes, starting with a letter or digit")
	}
	return nil
}

// vms returns the VMs built from limabean's templates.
func (a *App) vms(ctx context.Context) ([]lima.Instance, error) { return a.Lima.Limabean(ctx) }

// vm returns a VM by name, or an error pointing at `limabean ls`.
func (a *App) vm(ctx context.Context, name string) (lima.Instance, error) {
	inst, ok, err := a.Lima.Get(ctx, name)
	if err != nil {
		return inst, err
	}
	if !ok {
		return inst, fmt.Errorf("no VM named %q (see: limabean ls)", name)
	}
	return inst, nil
}

// vmArg returns args[0], or lets the user pick a VM in a terminal.
func (a *App) vmArg(ctx context.Context, args []string, verb string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	list, err := a.vms(ctx)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", errors.New("no VMs yet (limabean new <name>)")
	}
	if !a.UI.Prompting {
		return "", errors.New("name a VM (see: limabean ls)")
	}
	if len(list) == 1 {
		return list[0].Name, nil
	}
	names := make([]string, len(list))
	for i, inst := range list {
		names[i] = inst.Name
	}
	return a.UI.Select("VM to "+verb, names)
}

// ensureRunning starts a stopped VM and returns its current state.
func (a *App) ensureRunning(ctx context.Context, inst lima.Instance) (lima.Instance, error) {
	if inst.Running() {
		return inst, nil
	}
	if err := a.limaStep(ctx, "Starting "+inst.Name, func(ctx context.Context) ([]byte, error) {
		return a.Lima.Start(ctx, inst.Name)
	}); err != nil {
		return inst, err
	}
	return a.vm(ctx, inst.Name)
}

// limaStep runs a limactl call behind a spinner, showing Lima's output only
// if it fails.
func (a *App) limaStep(ctx context.Context, title string, fn func(context.Context) ([]byte, error)) error {
	var out []byte
	err := a.UI.Spin(ctx, title, func(ctx context.Context) error {
		var err error
		out, err = fn(ctx)
		return err
	})
	if err != nil {
		if len(out) > 0 {
			fmt.Fprintf(a.UI.Err, "%s\n", strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("%s: %w", strings.ToLower(title[:1])+title[1:], err)
	}
	return nil
}

// createOptions are the choices for a new VM.
type createOptions struct {
	Name, Env string
}

// create builds a VM, finishes its updates, installs its secrets and
// registers it with Claude Desktop.
func (a *App) create(ctx context.Context, o createOptions) error {
	root, err := a.templatesRoot()
	if err != nil {
		return err
	}
	tmpl, err := templates.Path(root, o.Env)
	if err != nil {
		return err
	}
	opts := lima.StartOptions{
		Name:     o.Name,
		Template: tmpl,
		Params:   map[string]string{"gitConfigDir": a.Cfg.GitConfigDir, "codeDir": a.Cfg.CodeDir},
		CPUs:     a.Cfg.CPUs,
		Memory:   a.Cfg.Memory,
	}
	// AWS CLI config, read-only: a writable mount would let the VM change
	// ~/.aws/config, whose credential_process runs commands on the Mac.
	aws := ""
	if a.Cfg.AWSDir != "" {
		if fi, err := os.Stat(config.Expand(a.Cfg.AWSDir)); err == nil && fi.IsDir() {
			aws = a.Cfg.AWSDir
			opts.Sets = append(opts.Sets, fmt.Sprintf(
				`.mounts += [{"location": %q, "mountPoint": "{{.Home}}/.aws", "writable": false}]`, aws))
		}
	}
	if err := a.limaStep(ctx, fmt.Sprintf("Creating %s from %s. The first boot takes a minute or two…", o.Name, o.Env),
		func(ctx context.Context) ([]byte, error) { return a.Lima.Create(ctx, opts) }); err != nil {
		return err
	}
	a.UI.OK("Created %s from %s", a.UI.Code(o.Name), o.Env)
	if aws != "" {
		a.UI.OK("Mounted %s read-only at ~/.aws", aws)
	}
	if err := a.saveEnv(o.Name, o.Env); err != nil {
		return err
	}
	inst, err := a.vm(ctx, o.Name)
	if err != nil {
		return err
	}

	// The first boot upgrades every package (features/apt-upgrade.sh). Some
	// updates, like a new kernel, only take effect after a restart.
	if _, err := a.Lima.Run(ctx, inst, nil, "test -f /var/run/reboot-required"); err == nil {
		if err := a.limaStep(ctx, "Restarting "+o.Name+" to finish installing updates", func(ctx context.Context) ([]byte, error) {
			if out, err := a.Lima.Stop(ctx, o.Name); err != nil {
				return out, err
			}
			return a.Lima.Start(ctx, o.Name)
		}); err != nil {
			return err
		}
		a.UI.OK("Installed updates and restarted")
	} else {
		a.UI.OK("Packages up to date")
	}

	if n, err := a.installSecrets(ctx, inst); err != nil {
		a.UI.Warn("Installed %d secret(s); fix the ones above, then: limabean secrets sync %s", n, o.Name)
	} else if n > 0 {
		a.UI.OK("Installed %d secret(s)", n)
	} else {
		a.UI.Warn("No secrets set up, so gh and other tools in the VM aren't logged in")
		a.UI.Hint("limabean github login   or   limabean secrets set NAME")
	}
	if a.Cfg.ClaudeDesktop {
		if err := a.Claude.Add(o.Name); err != nil {
			a.UI.Warn("Couldn't add %s to Claude Desktop: %v", o.Name, err)
		} else {
			a.UI.OK("Added to Claude Desktop")
		}
	}
	return nil
}

// installSecrets resolves the VM's secrets on the Mac and writes them into
// it, reporting each one that couldn't be resolved. The rest are installed
// either way.
func (a *App) installSecrets(ctx context.Context, inst lima.Instance) (int, error) {
	list, parseErrs := secrets.Load(a.Cfg.Dir(), inst.Name)
	for _, e := range parseErrs {
		a.UI.Warn("%v", e)
	}
	values, errs := a.Resolver.ResolveAll(ctx, list)
	for _, s := range list {
		if err := errs[s.Name]; err != nil {
			a.UI.Bad("%s (%s): %v", s.Name, s.Source, err)
		}
	}
	// Secrets that couldn't be resolved keep the value the VM already has, so
	// an env: reference doesn't vanish because one sync ran in a shell
	// without the variable.
	var keep []string
	for _, s := range list {
		if errs[s.Name] != nil {
			keep = append(keep, s.Name)
		}
	}
	if _, err := a.Lima.Run(ctx, inst, strings.NewReader(secrets.Render(values)), installScript(keep)); err != nil {
		return 0, fmt.Errorf("writing secrets to %s: %w", inst.Name, err)
	}
	if len(keep) > 0 {
		a.UI.Hint(fmt.Sprintf("%s kept its current value of %s, if it had one.", inst.Name, strings.Join(keep, ", ")))
	}
	if len(errs) > 0 {
		return len(values), fmt.Errorf("%d secret(s) couldn't be resolved", len(errs))
	}
	return len(values), nil
}

// installScript writes the VM's secrets file from stdin, atomically and
// mode 600. Variables named in keep are carried over from the old file (they
// couldn't be resolved this time). It also removes the bash version's token
// file and switches git's GitHub transport to match the token's presence.
// Names are validated identifiers, so they're safe to put in the script.
func installScript(keep []string) string {
	return `umask 077 && mkdir -p ~/.config/limabean && f=~/.config/limabean/secrets.env &&
cat > "$f.new" &&
for n in ` + strings.Join(keep, " ") + `; do
  ( [ -r "$f" ] && . "$f" >/dev/null 2>&1
    eval "[ -n \"\${$n+x}\" ]" || exit 0
    eval "v=\$$n"
    printf "export %s='%s'\n" "$n" "$(printf '%s' "$v" | sed "s/'/'\\\\''/g")" ) >> "$f.new"
done &&
mv "$f.new" "$f" &&
rm -f ~/.config/limabean/github-token &&
{ ! command -v limabean-github-git >/dev/null || limabean-github-git; }`
}

// syncSecrets installs secrets into one VM (starting it if needed) or, with
// name == "", into every running VM.
func (a *App) syncSecrets(ctx context.Context, name string) error {
	var targets []lima.Instance
	if name != "" {
		inst, err := a.vm(ctx, name)
		if err != nil {
			return err
		}
		if inst, err = a.ensureRunning(ctx, inst); err != nil {
			return err
		}
		targets = []lima.Instance{inst}
	} else {
		list, err := a.vms(ctx)
		if err != nil {
			return err
		}
		var stopped []string
		for _, inst := range list {
			if inst.Running() {
				targets = append(targets, inst)
			} else {
				stopped = append(stopped, inst.Name)
			}
		}
		if len(stopped) > 0 {
			defer func() {
				a.UI.Warn("Stopped, so not updated: %s", strings.Join(stopped, ", "))
				a.UI.Hint("Start them (limabean ssh <name>), then: limabean secrets sync")
			}()
		}
		if len(targets) == 0 && len(stopped) == 0 {
			a.UI.Hint("No VMs yet. New ones get their secrets automatically.")
			return nil
		}
	}
	failed := false
	for _, inst := range targets {
		n, err := a.installSecrets(ctx, inst)
		if err != nil {
			a.UI.Warn("%s: installed %d secret(s); %v", inst.Name, n, err)
			failed = true
			continue
		}
		a.UI.OK("%s: %d secret(s)", a.UI.Code(inst.Name), n)
	}
	if failed {
		return errors.New("some secrets weren't installed")
	}
	return nil
}

// checkUnsaved lists git repositories under the VM's code directory with
// uncommitted changes, stashes, or commits not on any remote. It returns
// false if there are any.
func (a *App) checkUnsaved(ctx context.Context, inst lima.Instance) (bool, error) {
	inst, err := a.ensureRunning(ctx, inst)
	if err != nil {
		return false, err
	}
	script := `code=` + secrets.ShellQuote(a.Cfg.CodeDir) + `; code=${code/#\~/$HOME}
[ -d "$code" ] || exit 0
find "$code" -maxdepth 6 -type d -name .git -prune 2>/dev/null | while read -r git; do
  repo=${git%/.git}; problems=""
  [ -n "$(git -C "$repo" status --porcelain 2>/dev/null)" ] && problems="$problems, uncommitted changes"
  [ -n "$(git -C "$repo" stash list 2>/dev/null)" ] && problems="$problems, stashes"
  [ -n "$(git -C "$repo" log --branches --not --remotes --oneline 2>/dev/null | head -1)" ] && problems="$problems, unpushed commits"
  [ -z "$problems" ] || echo "${repo/#$HOME/~}: ${problems#, }"
done`
	out, err := a.Lima.Run(ctx, inst, strings.NewReader(script), "bash -s")
	if err != nil {
		return false, fmt.Errorf("couldn't check %s for unsaved work (use --force to skip the check): %w", inst.Name, err)
	}
	report := strings.TrimSpace(string(out))
	if report == "" {
		a.UI.OK("No unsaved work in %s", inst.Name)
		return true, nil
	}
	a.UI.Bad("%s has work that would be lost:", inst.Name)
	a.UI.Hint(strings.Split(report, "\n")...)
	a.UI.Hint("Commit and push it, or pass --force to delete anyway.")
	return false, nil
}

// prune removes Claude Desktop entries and saved state for VMs deleted
// outside limabean, e.g. with `limactl delete`.
func (a *App) prune(ctx context.Context) {
	list, err := a.Lima.List(ctx)
	if err != nil {
		return
	}
	exists := map[string]bool{}
	for _, inst := range list {
		exists[inst.Name] = true
	}
	if a.Cfg.ClaudeDesktop {
		if names, err := a.Claude.VMs(); err == nil {
			for _, name := range names {
				if !exists[name] {
					if a.Claude.Remove(name) == nil {
						a.UI.Info("Removed %s from Claude Desktop (the VM no longer exists)", name)
					}
				}
			}
		}
	}
	entries, _ := os.ReadDir(a.stateDir)
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".env"); ok && !exists[name] {
			_ = os.Remove(filepath.Join(a.stateDir, e.Name()))
		}
	}
}

// saveEnv remembers which environment a VM was built from, for rebuild.
func (a *App) saveEnv(name, env string) error {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.stateDir, name+".env"), []byte(env+"\n"), 0o644)
}

// envOf returns the environment a VM was built from ("scratch" if unknown).
func (a *App) envOf(name string) string {
	b, err := os.ReadFile(filepath.Join(a.stateDir, name+".env"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// forget removes everything limabean keeps about a deleted VM: its saved
// environment, its Claude Desktop entry, its own secrets file and the
// Keychain items limabean created for it.
func (a *App) forget(name string) {
	_ = os.Remove(filepath.Join(a.stateDir, name+".env"))
	if a.Cfg.ClaudeDesktop {
		if err := a.Claude.Remove(name); err != nil && !errors.Is(err, claude.ErrInvalid) {
			a.UI.Warn("Couldn't update Claude Desktop: %v", err)
		}
	}
	file := secrets.VMFile(a.Cfg.Dir(), name)
	list, _ := secrets.LoadFile(file)
	for _, s := range list {
		if a.ownsKeychainRef(s.Ref) && !a.referencedElsewhere(s.Ref, file) {
			_ = a.Keychain.Delete(strings.TrimPrefix(s.Ref, "keychain:"))
		}
	}
	if err := os.Remove(file); err == nil {
		a.UI.OK("Removed %s's own secrets (%s)", name, filepath.Base(file))
	}
}

// attach runs ssh in the VM attached to the terminal (unlike Exec, it
// returns, so limabean can carry on afterwards).
func attach(inst lima.Instance, command ...string) error {
	cmd := exec.Command("ssh", lima.SSHArgs(inst, false, command...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// completeVMs tab-completes the first argument with VM names.
func completeVMs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	list, err := lima.New().Limabean(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(list))
	for _, inst := range list {
		names = append(names, inst.Name+"\t"+inst.Status)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// hasLeftovers reports whether limabean still keeps something for a VM
// that no longer exists.
func (a *App) hasLeftovers(name string) bool {
	if _, err := os.Stat(secrets.VMFile(a.Cfg.Dir(), name)); err == nil {
		return true
	}
	if a.Cfg.ClaudeDesktop {
		if names, err := a.Claude.VMs(); err == nil && slices.Contains(names, name) {
			return true
		}
	}
	return false
}

// referencedElsewhere reports whether any secrets file other than skip
// still uses ref, so its Keychain item must be kept.
func (a *App) referencedElsewhere(ref, skip string) bool {
	files, _ := filepath.Glob(filepath.Join(a.Cfg.Dir(), "secrets*.env"))
	for _, f := range files {
		if f == skip {
			continue
		}
		list, _ := secrets.LoadFile(f)
		for _, s := range list {
			if s.Ref == ref {
				return true
			}
		}
	}
	return false
}

// shortPath shows a path with ~ for the home directory.
func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

// vmHasSecret reports whether a running VM has a secret installed.
func (a *App) vmHasSecret(ctx context.Context, inst lima.Instance, name string) bool {
	_, err := a.Lima.Run(ctx, inst, nil, "grep -q '^export "+name+"=' ~/.config/limabean/secrets.env")
	return err == nil
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-len(s))
}

// contextT keeps spinner callbacks short.
type contextT = context.Context
