package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/secrets"
	"github.com/rossedman/limabean/internal/templates"
)

func newNewCmd(a *App) *cobra.Command {
	var nf newFlags
	cmd := &cobra.Command{
		Use:     "new [name] [environment]",
		Aliases: []string{"create"},
		Hidden:  true, // "limabean up <name>" creates a VM; new stays for scripts
		Short:   "Create a VM (limabean up <name> does this now)",
		Long: `Create a VM from an environment template (default: scratch), boot it, apply
every pending Ubuntu update, install its secrets and register it with
Claude Desktop. The first VM of a kind (environment and packs) is
provisioned from scratch, which takes a few minutes, and an image of it is
built in the background; later ones are copies of that image and only
have to boot (see: limabean images --help).

A VM gets the core shell tools, Docker and Python, plus the packs you pick:
in a terminal, new asks which (the PACKS setting is checked already), and
--with and --without choose them on the command line (see: limabean packs).

In a terminal, new asks for anything you didn't pass and then shows the
secrets step:
  - shared secrets (secrets.env) are listed; every VM gets them
  - secrets from your other VMs can be added to this one
  - "Add a secret" saves a new one: paste a value into the Keychain, or
    point at 1Password, an environment variable or a command, for this VM
    or for every VM

Without a terminal (or with --plain) nothing is asked: pass the name, and
use --secret to give the VM its own secrets.

Your own environments go in ~/.config/limabean/templates/<env>.yaml and can
build on the built-in layers with: base: [./layer-base.yaml]`,
		Example: `  # Answer a few questions, including which secrets to add
  limabean new

  # Name it; the form still offers the secrets step
  limabean new dev

  # From a script: no questions, two secrets just for this VM
  limabean new ci --plain \
    --secret OPENAI_API_KEY=op://Private/OpenAI/credential \
    --secret NPM_TOKEN=env:NPM_TOKEN

  # Bigger than the template default (2 CPUs, 4 GiB, a 10 GiB disk)
  limabean new k8s --cpus 8 --memory 16 --disk 40

  # Record what it reaches, then allow just that (limabean network learn)
  limabean new agent --learn

  # A throwaway VM, deleted in four hours
  limabean new spike --ttl 4h

  # Go and Postgres's client, without the questions about packs
  limabean new api --with go,db

  # An agent sandbox: GitHub and PyPI only, and no way to undo that
  limabean new agent --allow github --allow python --locked`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: completeEnvs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.newVM(cmd, args, &nf)
		},
	}
	nf.add(cmd.Flags())
	_ = a.V.BindPFlag(config.KeyCPUs, cmd.Flags().Lookup("cpus"))
	_ = a.V.BindPFlag(config.KeyMemory, cmd.Flags().Lookup("memory"))
	_ = a.V.BindPFlag(config.KeyDisk, cmd.Flags().Lookup("disk"))
	return cmd
}

// newFlags are the choices for a VM made without a project file, as flags
// on up (and on the older new).
type newFlags struct {
	secrets, allow, with, without []string
	locked, learn                 bool
	ttl                           time.Duration
}

func (nf *newFlags) add(f *pflag.FlagSet) {
	f.StringArrayVar(&nf.secrets, "secret", nil, "give this VM a secret: NAME=reference (repeatable)")
	f.StringSliceVar(&nf.with, "with", nil, "give the VM these packs, e.g. go,node (see: limabean packs)")
	f.StringSliceVar(&nf.without, "without", nil, "leave out packs the PACKS setting would add")
	f.StringArrayVar(&nf.allow, "allow", nil, "only let the VM reach this domain or preset (repeatable; see: limabean network --help)")
	f.BoolVar(&nf.learn, "learn", false, "record the domains the VM looks up, to build its allowlist (see: limabean network learn --help)")
	f.BoolVar(&nf.locked, "locked", false, "take sudo and Docker away from the VM's user, so its allowlist holds")
	f.DurationVar(&nf.ttl, "ttl", 0, "delete the VM, whatever is in it, once it's this old, e.g. 2h (needs: limabean autostop on)")
	f.Int("cpus", 0, "CPUs for the VM (default: the template's, 2)")
	f.String("memory", "", "memory in GiB (default: the template's, 4)")
	f.String("disk", "", "system disk in GiB (default: the template's, 10); ~/code has its own")
}

// newOnly are the flags that only mean something when a VM is created
// without a project file.
var newOnly = []string{"secret", "with", "without", "allow", "learn", "locked", "ttl", "cpus", "memory", "disk"}

// changedFlags lists which of names were given on the command line.
func changedFlags(cmd *cobra.Command, names []string) []string {
	var set []string
	for _, n := range names {
		if cmd.Flags().Changed(n) {
			set = append(set, "--"+n)
		}
	}
	return set
}

// newVM creates a VM from flags and, in a terminal, a form: name and
// environment if missing, packs, then secrets.
func (a *App) newVM(cmd *cobra.Command, args []string, nf *newFlags) error {
	ctx := cmd.Context()
	o := createOptions{}
	if len(args) > 0 {
		o.Name = args[0]
	}
	if len(args) > 1 {
		o.Env = args[1]
	}
	root, err := a.templatesRoot()
	if err != nil {
		return err
	}
	envs, err := templates.Envs(root)
	if err != nil {
		return err
	}
	if o.Name == "" && !a.UI.Prompting {
		return errors.New("name the VM: limabean up <name>")
	}

	flagSecrets, err := parseSecretFlags(nf.secrets)
	if err != nil {
		return err
	}
	if a.UI.Prompting {
		var packs []templates.Pack
		if !cmd.Flags().Changed("with") && !cmd.Flags().Changed("without") {
			if packs, err = templates.Packs(root); err != nil {
				return err
			}
		}
		picked, err := a.newForm(&o, envs, packs, len(flagSecrets) > 0)
		if err != nil {
			return err
		}
		if packs != nil {
			nf.with, nf.without = packChoice(templates.SplitList(a.Cfg.Packs), picked)
		}
	}
	if o.Env == "" {
		o.Env = "scratch"
	}
	if err := validVMName(o.Name); err != nil {
		return fmt.Errorf("VM name %q: %w", o.Name, err)
	}
	if _, exists, err := a.Lima.Get(ctx, o.Name); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("a VM named %q already exists", o.Name)
	}
	if err := a.prepareVM(&o, vmChoices{secrets: flagSecrets, with: nf.with, without: nf.without, allow: nf.allow, locked: nf.locked, learn: nf.learn}); err != nil {
		return err
	}

	a.prune(ctx)
	if err := a.create(ctx, o); err != nil {
		return err
	}
	if err := a.setTTL(ctx, o.Name, nf.ttl); err != nil {
		return err
	}
	lines := []string{a.UI.Code(o.Name) + " is ready", "", "ssh lima-" + o.Name + "     or: limabean ssh " + o.Name}
	if a.Cfg.ClaudeDesktop {
		lines = append(lines, "Claude Desktop: SSH → "+o.Name+" (limabean)")
	}
	a.UI.Box(lines...)
	return nil
}

// newForm asks for the name and environment if missing and, given packs,
// which of them the VM gets; then which secrets it gets. skipSecrets is set
// when --secret flags already chose them. It returns the packs picked.
func (a *App) newForm(o *createOptions, envs []string, packs []templates.Pack, skipSecrets bool) ([]string, error) {
	var fields []huh.Field
	var picked []string
	if o.Name == "" {
		fields = append(fields, huh.NewInput().Title("Name").
			Description("Also its SSH host: ssh lima-<name>").
			Placeholder("dev").Value(&o.Name).
			Validate(func(v string) error { return validVMName(strings.TrimSpace(v)) }))
	}
	if o.Env == "" && len(envs) > 1 {
		o.Env = "scratch"
		opts := make([]huh.Option[string], len(envs))
		for i, e := range envs {
			opts[i] = huh.NewOption(e, e)
		}
		fields = append(fields, huh.NewSelect[string]().Title("Environment").Options(opts...).Value(&o.Env))
	}
	if opts := packOptions(packs, templates.SplitList(a.Cfg.Packs)); len(opts) > 0 {
		fields = append(fields, huh.NewMultiSelect[string]().Title("Packs").
			Description("Besides core shell tools, Docker and Python · space to select, enter to continue").
			Options(opts...).Value(&picked))
	}
	if len(fields) > 0 {
		if err := a.UI.Run(huh.NewGroup(fields...)); err != nil {
			return nil, err
		}
		o.Name = strings.TrimSpace(o.Name)
	}
	if skipSecrets {
		return picked, nil
	}
	return picked, a.secretsStep(o.Name)
}

// packOptions are the packs a VM can get besides core, the defaults
// selected.
func packOptions(packs []templates.Pack, defaults []string) []huh.Option[string] {
	var opts []huh.Option[string]
	for _, p := range packs {
		if p.Name == templates.CorePack {
			continue
		}
		opts = append(opts, huh.NewOption(fmt.Sprintf("%-12s %s", p.Name, p.Description), p.Name).Selected(slices.Contains(defaults, p.Name)))
	}
	return opts
}

// packChoice turns the packs picked in the form into --with and --without
// against the defaults.
func packChoice(defaults, picked []string) (with, without []string) {
	for _, d := range defaults {
		if !slices.Contains(picked, d) {
			without = append(without, d)
		}
	}
	return picked, without
}

// secretsStep shows which secrets the new VM gets, offers ones from other
// VMs, and adds new ones, writing the choices to the VM's own file.
func (a *App) secretsStep(vm string) error {
	shared, _ := secrets.Load(a.Cfg.Dir(), "")
	otherNames, otherRefs := a.otherVMSecrets(vm)

	sharedDesc := "None yet."
	if len(shared) > 0 {
		var lines []string
		for _, s := range shared {
			lines = append(lines, fmt.Sprintf("%s  ·  %s", s.Name, secrets.Kind(s.Ref)))
		}
		sharedDesc = strings.Join(lines, "\n")
	}
	var picked []string
	addNew := len(shared) == 0 && len(otherNames) == 0
	fields := []huh.Field{
		huh.NewNote().Title("Secrets every VM gets").Description(sharedDesc),
	}
	if len(otherNames) > 0 {
		opts := make([]huh.Option[string], len(otherNames))
		for i, n := range otherNames {
			s := otherRefs[n]
			opts[i] = huh.NewOption(fmt.Sprintf("%s  ·  %s  (from %s)", n, secrets.Kind(s.Ref), strings.TrimSuffix(strings.TrimPrefix(s.Source, "secrets."), ".env")), n)
		}
		fields = append(fields, huh.NewMultiSelect[string]().
			Title("Also give "+vm+" secrets from your other VMs").
			Description("space to select, enter to continue").
			Options(opts...).Value(&picked))
	}
	fields = append(fields, huh.NewConfirm().Title("Add a new secret?").Value(&addNew))
	if err := a.UI.Run(huh.NewGroup(fields...)); err != nil {
		return err
	}
	for _, n := range picked {
		if err := secrets.Put(secrets.VMFile(a.Cfg.Dir(), vm), n, otherRefs[n].Ref); err != nil {
			return err
		}
	}
	for addNew {
		s := newSecret{Source: sourcePaste}
		if err := a.UI.Run(addSecretGroups(&s, vm)...); err != nil {
			return err
		}
		file, err := a.saveSecret(s, vm)
		if err != nil {
			return err
		}
		a.UI.OK("%s → %s", strings.TrimSpace(s.Name), shortPath(file))
		addNew = false
		if err := a.UI.Run(huh.NewGroup(huh.NewConfirm().Title("Add another secret?").Value(&addNew))); err != nil {
			return err
		}
	}
	return nil
}

// parseSecretFlags reads --secret NAME=reference values.
func parseSecretFlags(flags []string) ([]secrets.Secret, error) {
	var out []secrets.Secret
	for _, f := range flags {
		name, ref, ok := strings.Cut(f, "=")
		if !ok || !secrets.ValidName(name) || ref == "" {
			return nil, fmt.Errorf("--secret %q: expected NAME=reference, e.g. OPENAI_API_KEY=op://Private/OpenAI/credential", f)
		}
		out = append(out, secrets.Secret{Name: name, Ref: ref})
	}
	return out, nil
}

// completeEnvs tab-completes the second argument with environment names.
func completeEnvs(a *App) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 1 || a.setup() != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		root, err := a.templatesRoot()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		envs, _ := templates.Envs(root)
		return envs, cobra.ShellCompDirectiveNoFileComp
	}
}

// vmChoices are what a new VM gets besides its name and environment.
type vmChoices struct {
	secrets       []secrets.Secret // its own, as NAME=reference
	with, without []string         // packs, on top of the PACKS setting
	exact         bool             // with is every pack; PACKS doesn't apply (project files)
	allow         []string         // domains and presets; none: open
	locked, learn bool
}

// prepareVM saves what a VM gets before it's created: its own secrets, its
// network policy, and its packs, which it puts in o.
func (a *App) prepareVM(o *createOptions, c vmChoices) error {
	base := templates.SplitList(a.Cfg.Packs)
	if c.exact {
		base = nil
	}
	packs, err := a.resolvePacks(base, c.with, c.without)
	if err != nil {
		return err
	}
	o.Packs = packs
	domains, err := expandAllow(c.allow)
	if err != nil {
		return err
	}
	for _, s := range c.secrets {
		if err := secrets.Put(secrets.VMFile(a.Cfg.Dir(), o.Name), s.Name, s.Ref); err != nil {
			return err
		}
	}
	if len(domains) > 0 || c.locked || c.learn {
		return a.savePolicy(o.Name, netPolicy{Locked: c.locked, Learn: c.learn, Allow: domains})
	}
	return nil
}
