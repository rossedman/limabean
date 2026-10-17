package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/secrets"
	"github.com/rossedman/limabean/internal/templates"
)

func newNewCmd(a *App) *cobra.Command {
	var secretFlags, allowFlags, withPacks, withoutPacks []string
	var locked bool
	cmd := &cobra.Command{
		Use:     "new [name] [environment]",
		Aliases: []string{"create"},
		Short:   "Create a VM",
		Long: `Create a VM from an environment template (default: scratch), boot it, apply
every pending Ubuntu update, install its secrets and register it with
Claude Desktop. The first VM takes a few minutes while Lima downloads
Ubuntu; later ones take about a minute.

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

  # Bigger than the template default
  limabean new k8s --cpus 8 --memory 16

  # The default packs, plus Rust and Java, without Kubernetes
  limabean new api --with rust,java --without kubernetes

  # An agent sandbox: GitHub and PyPI only, and no way to undo that
  limabean new agent --allow github --allow python --locked`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: completeEnvs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return errors.New("name the VM: limabean new <name>")
			}

			flagSecrets, err := parseSecretFlags(secretFlags)
			if err != nil {
				return err
			}
			if a.UI.Prompting {
				if err := a.newForm(&o, envs, len(flagSecrets) > 0); err != nil {
					return err
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
			for _, s := range flagSecrets {
				if err := secrets.Put(secrets.VMFile(a.Cfg.Dir(), o.Name), s.Name, s.Ref); err != nil {
					return err
				}
			}
			packs, err := a.resolvePacks(templates.SplitList(a.Cfg.Packs), withPacks, withoutPacks)
			if err != nil {
				return err
			}
			o.Packs = packs
			if len(allowFlags) > 0 || locked {
				domains, err := expandAllow(allowFlags)
				if err != nil {
					return err
				}
				if err := a.savePolicy(o.Name, netPolicy{Locked: locked, Allow: domains}); err != nil {
					return err
				}
			}

			a.prune(ctx)
			if err := a.create(ctx, o); err != nil {
				return err
			}
			lines := []string{a.UI.Code(o.Name) + " is ready", "", "ssh lima-" + o.Name + "     or: limabean ssh " + o.Name}
			if a.Cfg.ClaudeDesktop {
				lines = append(lines, "Claude Desktop: SSH → "+o.Name+" (limabean)")
			}
			a.UI.Box(lines...)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringArrayVar(&secretFlags, "secret", nil, "give this VM a secret: NAME=reference (repeatable)")
	f.StringSliceVar(&withPacks, "with", nil, "add these packs to the defaults, e.g. rust,java (see: limabean packs)")
	f.StringSliceVar(&withoutPacks, "without", nil, "leave these packs out, e.g. kubernetes")
	f.StringArrayVar(&allowFlags, "allow", nil, "only let the VM reach this domain or preset (repeatable; see: limabean network --help)")
	f.BoolVar(&locked, "locked", false, "take sudo and Docker away from the VM's user, so its allowlist holds")
	f.Int("cpus", 0, "CPUs for the VM (default: the template's, 4)")
	f.String("memory", "", "memory in GiB (default: the template's, 8)")
	_ = a.V.BindPFlag(config.KeyCPUs, f.Lookup("cpus"))
	_ = a.V.BindPFlag(config.KeyMemory, f.Lookup("memory"))
	return cmd
}

// newForm asks for the name and environment if missing, then which secrets
// the VM gets. skipSecrets is set when --secret flags already chose them.
func (a *App) newForm(o *createOptions, envs []string, skipSecrets bool) error {
	var fields []huh.Field
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
	if len(fields) > 0 {
		if err := a.UI.Run(huh.NewGroup(fields...)); err != nil {
			return err
		}
		o.Name = strings.TrimSpace(o.Name)
	}
	if skipSecrets {
		return nil
	}
	return a.secretsStep(o.Name)
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
