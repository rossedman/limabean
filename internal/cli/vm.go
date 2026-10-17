package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	a.claudeAdd(inst.Name)
	return a.vm(ctx, inst.Name)
}

// claudeAdd and claudeRemove keep a VM's Claude Desktop entry in step with
// whether it runs: Claude Desktop shows an error for a connection whose VM
// is stopped, so only running VMs are listed.
func (a *App) claudeAdd(name string) {
	if !a.Cfg.ClaudeDesktop {
		return
	}
	if err := a.Claude.Add(name); err != nil {
		a.UI.Warn("Couldn't add %s to Claude Desktop: %v", name, err)
		return
	}
	a.UI.OK("Added to Claude Desktop")
}

func (a *App) claudeRemove(name string) {
	if !a.Cfg.ClaudeDesktop {
		return
	}
	if err := a.Claude.Remove(name); err != nil && !errors.Is(err, claude.ErrInvalid) {
		a.UI.Warn("Couldn't update Claude Desktop: %v", err)
	}
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
	Packs     []string // besides core, which layer-base installs
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
	packSets, err := packProvision(root, o.Packs)
	if err != nil {
		return err
	}
	opts := lima.StartOptions{
		Name:     o.Name,
		Template: tmpl,
		Params:   map[string]string{"gitConfigDir": a.Cfg.GitConfigDir, "codeDir": a.Cfg.CodeDir, "networkDir": a.networkDir(o.Name)},
		CPUs:     a.Cfg.CPUs,
		Memory:   a.Cfg.Memory,
	}
	if err := os.MkdirAll(a.networkDir(o.Name), 0o755); err != nil {
		return err
	}
	disk, err := a.prepareCodeDisk(ctx, o.Name)
	if err != nil {
		return err
	}
	opts.Sets = append(opts.Sets, packSets...)
	if disk != "" {
		opts.Params["codeDisk"] = disk
		opts.Sets = append(opts.Sets, fmt.Sprintf(`.additionalDisks += [{"name": %q, "format": true, "fsType": "ext4"}]`, disk))
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
	if disk != "" {
		a.UI.OK("%s is on its own disk, kept when the VM is rebuilt", a.Cfg.CodeDir)
	}
	if aws != "" {
		a.UI.OK("Mounted %s read-only at ~/.aws", aws)
	}
	if err := a.saveEnv(o.Name, o.Env); err != nil {
		return err
	}
	if err := a.savePacks(o.Name, o.Packs); err != nil {
		return err
	}
	if len(o.Packs) > 0 {
		a.UI.OK("Packs: core, %s", strings.Join(o.Packs, ", "))
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
	// The first boot provisions with the network open; the VM's policy
	// (from new --allow/--locked, or kept through a rebuild) applies now.
	policy, err := a.loadPolicy(o.Name)
	if err != nil {
		return err
	}
	if !policy.open() || policy.Locked {
		if inst, err = a.vm(ctx, o.Name); err != nil {
			return err
		}
		if err := a.applyPolicy(ctx, inst); err != nil {
			return err
		}
		a.UI.OK("Network: %s", policy.summary())
	}
	a.claudeAdd(o.Name)
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

// prune brings Claude Desktop in line with VMs started, stopped or deleted
// outside limabean (limactl, a Mac restart): entries go for VMs that are
// gone or stopped, and running VMs get one. Saved state for deleted VMs is
// removed too.
func (a *App) prune(ctx context.Context) {
	list, err := a.Lima.List(ctx)
	if err != nil {
		return
	}
	exists, running := map[string]bool{}, map[string]bool{}
	for _, inst := range list {
		exists[inst.Name] = true
		running[inst.Name] = inst.Limabean && inst.Running()
	}
	if a.Cfg.ClaudeDesktop {
		if names, err := a.Claude.VMs(); err == nil {
			for _, name := range names {
				if running[name] || a.Claude.Remove(name) != nil {
					continue
				}
				if exists[name] {
					a.UI.Info("Removed %s from Claude Desktop (the VM is stopped)", name)
				} else {
					a.UI.Info("Removed %s from Claude Desktop (the VM no longer exists)", name)
				}
			}
			for _, inst := range list {
				if running[inst.Name] && !slices.Contains(names, inst.Name) && a.Claude.Add(inst.Name) == nil {
					a.UI.Info("Added %s to Claude Desktop (the VM is running)", inst.Name)
				}
			}
		}
	}
	entries, _ := os.ReadDir(a.stateDir)
	for _, e := range entries {
		name := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".env"), ".packs")
		if name != e.Name() && !exists[name] {
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
// environment, its Claude Desktop entry, its code disk, its own secrets
// file and the Keychain items limabean created for it.
func (a *App) forget(ctx context.Context, name string) {
	_ = os.Remove(filepath.Join(a.stateDir, name+".env"))
	_ = os.Remove(filepath.Join(a.stateDir, name+".packs"))
	_ = os.RemoveAll(a.networkDir(name))
	if disk := codeDiskName(name); a.hasCodeDisk(ctx, name) {
		if out, err := a.Lima.DiskDelete(ctx, disk); err != nil {
			a.UI.Warn("Couldn't delete %s's code disk %s: %v %s", name, disk, err, strings.TrimSpace(string(out)))
			a.UI.Hint("limactl disk delete " + disk)
		} else {
			a.UI.OK("Deleted %s's code disk", name)
		}
	}
	a.claudeRemove(name)
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

// completeVMs tab-completes the first argument with VM names. It runs
// before setup, so it makes its own Lima client unless one is set.
func (a *App) completeVMs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client := a.Lima
	if client == nil {
		client = lima.New()
	}
	list, err := client.Limabean(context.Background())
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
func (a *App) hasLeftovers(ctx context.Context, name string) bool {
	if a.hasCodeDisk(ctx, name) {
		return true
	}
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

// codeDiskName is the Lima disk that holds a VM's code directory. Lima
// labels a disk's filesystem lima-<name> and formats it on any boot that
// can't find that label, and ext4 labels hold 16 bytes, so the name must
// stay within 11: a longer one would be cut short and the disk erased on
// every boot. A hash of the VM name keeps it short and unique.
func codeDiskName(vm string) string {
	sum := sha256.Sum256([]byte(vm))
	return "code-" + hex.EncodeToString(sum[:3])
}

// hasCodeDisk reports whether a VM has a code disk (false if Lima can't
// say).
func (a *App) hasCodeDisk(ctx context.Context, vm string) bool {
	ok, err := a.Lima.DiskExists(ctx, codeDiskName(vm))
	return err == nil && ok
}

// prepareCodeDisk returns the disk to mount over a new VM's code
// directory: the one it already has (after a rebuild), or a new one of
// CODE_DISK GiB. "" means none: CODE_DISK is off and there's no disk.
func (a *App) prepareCodeDisk(ctx context.Context, vm string) (string, error) {
	disk := codeDiskName(vm)
	ok, err := a.Lima.DiskExists(ctx, disk)
	if err != nil {
		return "", err
	}
	if ok {
		return disk, nil
	}
	if a.Cfg.CodeDisk == 0 {
		return "", nil
	}
	size := fmt.Sprintf("%dGiB", a.Cfg.CodeDisk)
	if err := a.limaStep(ctx, fmt.Sprintf("Creating a %s disk for %s", size, a.Cfg.CodeDir), func(ctx context.Context) ([]byte, error) {
		return a.Lima.DiskCreate(ctx, disk, size)
	}); err != nil {
		return "", err
	}
	return disk, nil
}

// packProvision returns the limactl --set expressions that write the packs'
// mise TOMLs and apt lists into a new VM.
func packProvision(root string, names []string) ([]string, error) {
	all, err := templates.Packs(root)
	if err != nil {
		return nil, err
	}
	var sets []string
	for _, name := range names {
		i := slices.IndexFunc(all, func(p templates.Pack) bool { return p.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("unknown pack %q (see: limabean packs)", name)
		}
		add := func(path, content string) {
			c, _ := json.Marshal(content)
			sets = append(sets, fmt.Sprintf(`.provision += [{"mode": "data", "path": %q, "content": %s}]`, path, c))
		}
		add("/etc/limabean/mise/50-"+name+".toml", all[i].TOML)
		if list := all[i].AptList(); list != "" {
			add("/etc/limabean/apt/"+name+".list", list)
		}
	}
	return sets, nil
}

// resolvePacks works out a VM's packs: base (its saved packs, or the PACKS
// setting), plus with, minus without.
func (a *App) resolvePacks(base, with, without []string) ([]string, error) {
	root, err := a.templatesRoot()
	if err != nil {
		return nil, err
	}
	all, err := templates.Packs(root)
	if err != nil {
		return nil, err
	}
	return templates.ResolvePacks(all, base, with, without)
}

// savePacks remembers which packs a VM was built with, for rebuild.
func (a *App) savePacks(name string, packs []string) error {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.stateDir, name+".packs"), []byte(strings.Join(packs, ",")+"\n"), 0o644)
}

// packsOf returns the packs a VM was built with, and whether that's known
// (VMs made before packs aren't).
func (a *App) packsOf(name string) ([]string, bool) {
	b, err := os.ReadFile(filepath.Join(a.stateDir, name+".packs"))
	if err != nil {
		return nil, false
	}
	return templates.SplitList(strings.TrimSpace(string(b))), true
}
