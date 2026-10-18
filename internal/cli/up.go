package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/secrets"
)

func newUpCmd(a *App) *cobra.Command {
	var file string
	var yes, setup bool
	var nf newFlags
	cmd := &cobra.Command{
		Use:   "up [name] [environment]",
		Short: "Create or start a VM, or bring a project's VM up to date",
		Long: `Bring up a VM: up is how VMs are made and started.

In a project, up works from its .limabean.yaml (or .limabean.yml), here or
in a parent directory. It creates the VM the file describes, clones the
repository into it and runs its setup. If the VM exists, up starts it and
applies whatever in the file changed: new packs or environment rebuild it
(keeping ~/code), a new size resizes it, new secrets and Claude plugins are
added, and a tighter network is applied. A file can't widen a VM's
network: up prints the command for you to run instead. Setup runs once;
--setup runs it again.

With a name, up starts that VM, or creates it if there's none: in a
terminal it asks which packs and secrets it gets, and --with, --allow,
--locked, --ttl and the size flags choose on the command line. Inside a
project, the project's own name (or no name) means its file; any other
name is a VM of its own. Without a terminal, name the VM.

limabean down stops a VM; limabean rm deletes it.

  # .limabean.yaml
  name: myapp                 # default: the directory's name
  env: scratch                # environment template
  packs: [go, db, claude]     # besides core, Docker and Python (limabean packs)
  cpus: 6
  memory: 12                  # GiB
  disk: 30                    # GiB of system disk
  network:                    # see: limabean network --help
    allow: [github, npm]
    locked: true
  secrets: [OPENAI_API_KEY]   # names only; values stay on your Mac
  ports: [3000]               # shown as http://localhost:<port> after up
  claude:                     # plugins for Claude Code in the VM
    marketplaces:
      - marketplace: rossedman/guardrails     # GitHub owner/repo, or an https git URL
        plugins: [guardrails@guardrails]      # plugin@marketplace
  setup: |                    # run once in the clone, with bash -e
    make deps

The repository cloned is the checkout's origin (or repo: in the file).
Secrets are names only: each must already be set up on your Mac (limabean
secrets ls), and shared secrets are included as always.

Claude plugins go into the VM's ~/.claude/settings.json, and Claude Code
fetches them in the VM when a session starts, so a VM with a network
allowlist needs github (or the marketplace's host).

The file comes with the repository, so up shows what it asks for, or what
it would change, and waits for you to confirm; --yes skips that, for files
you trust.`,
		Example: `  # The project you're in: create it, or start it and apply the file's changes
  cd ~/src/myapp && limabean up

  # Run the project's setup again
  limabean up --setup

  # A VM of its own: start dev, or create it (asking which packs and secrets)
  limabean up dev

  # Create one from a script: Go and Node, 4 CPUs, gone in a day
  limabean up scratch --with go,node --cpus 4 --ttl 24h

  # An agent sandbox: GitHub and PyPI only, and no way to undo that
  limabean up agent --with python,claude --allow github --allow python --locked`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: a.completeUp,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path, f, err := findProject(file)
			if err != nil {
				return err
			}
			given := changedFlags(cmd, newOnly)
			if f != nil && (len(args) == 0 || args[0] == f.Name) {
				switch {
				case len(args) > 1:
					return fmt.Errorf("%s sets %s's environment; change it there", shortPath(path), f.Name)
				case len(given) > 0:
					return fmt.Errorf("%s choose what a VM without a project file gets; %s describes %s, so change it there", strings.Join(given, ", "), shortPath(path), f.Name)
				}
				if inst, ok, err := a.Lima.Get(ctx, f.Name); err != nil {
					return err
				} else if ok {
					return a.upExisting(ctx, inst, f, filepath.Dir(path), yes, setup)
				}
				return a.upCreate(ctx, f, filepath.Dir(path), yes)
			}
			if setup {
				return errors.New("--setup runs a project file's setup; there's no .limabean.yaml here")
			}
			if len(args) == 0 && !a.UI.Prompting {
				return errors.New("no .limabean.yaml here or in a parent directory; name a VM to start or create: limabean up <name>")
			}
			if len(args) > 0 {
				if err := validVMName(args[0]); err != nil {
					return fmt.Errorf("VM name %q: %w", args[0], err)
				}
				inst, ok, err := a.Lima.Get(ctx, args[0])
				if err != nil {
					return err
				}
				if ok {
					if len(args) > 1 {
						given = append(given, "an environment")
					}
					if len(given) > 0 {
						return fmt.Errorf("%s already exists, and %s only apply when creating a VM (packs: limabean rebuild %s --with …; network: limabean network)", args[0], strings.Join(given, ", "), args[0])
					}
					return a.upStart(ctx, inst)
				}
			}
			if err := a.sizeFlags(cmd); err != nil {
				return err
			}
			return a.newVM(cmd, args, &nf)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "the project file (default: .limabean.yaml here or in a parent directory)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "create the project's VM, or apply changes to it, without asking")
	cmd.Flags().BoolVar(&setup, "setup", false, "run the project file's setup again in its existing VM")
	nf.add(cmd.Flags())
	return cmd
}

// findProject loads the project file: the one named, or the one found here
// or in a parent directory. Without either it returns a nil file.
func findProject(file string) (string, *project.File, error) {
	path := file
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", nil, err
		}
		if path, err = project.Find(wd); errors.Is(err, project.ErrNotFound) {
			return "", nil, nil
		} else if err != nil {
			return "", nil, err
		}
	}
	f, err := project.Load(path)
	return path, f, err
}

// upStart starts an existing VM that isn't a project's.
func (a *App) upStart(ctx context.Context, inst lima.Instance) error {
	if inst.Running() {
		a.claudeAdd(inst.Name)
		a.UI.Info("%s is already running", inst.Name)
		return nil
	}
	if _, err := a.ensureRunning(ctx, inst); err != nil {
		return err
	}
	a.UI.OK("Started %s", a.UI.Code(inst.Name))
	return nil
}

// sizeFlags applies up's --cpus, --memory and --disk. new binds its own
// to the settings; a setting takes one flag, so up's are applied here.
func (a *App) sizeFlags(cmd *cobra.Command) error {
	f := cmd.Flags()
	if f.Changed("cpus") {
		n, _ := f.GetInt("cpus")
		a.Cfg.CPUs = n
	}
	if f.Changed("memory") {
		a.Cfg.Memory, _ = f.GetString("memory")
	}
	if f.Changed("disk") {
		v, _ := f.GetString("disk")
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fmt.Errorf("--disk %q: a size in GiB, e.g. 30", v)
		}
		a.Cfg.Disk = n
	}
	return nil
}

// completeUp completes a VM's name, then an environment.
func (a *App) completeUp(cmd *cobra.Command, args []string, s string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 1 {
		return completeEnvs(a)(cmd, args, s)
	}
	return a.completeVMs(cmd, args, s)
}

// upCreate creates a project's VM, clones the project and runs its setup.
func (a *App) upCreate(ctx context.Context, f *project.File, dir string, yes bool) error {
	repo := a.projectRepo(ctx, f, dir)
	own, missing := a.projectSecrets(f.Secrets)
	a.UI.Section(f.Name, "from "+shortPath(filepath.Join(dir, project.FileName)))
	plan := []string{"environment: " + firstNonEmpty(f.Env, "scratch")}
	plan = append(plan, "packs: "+strings.Join(append([]string{"core"}, f.Packs...), ", "))
	if f.CPUs > 0 || f.Memory > 0 || f.Disk > 0 {
		plan = append(plan, fmt.Sprintf("size: %d CPUs, %d GiB of memory, a %d GiB disk (0: the default)", f.CPUs, f.Memory, f.Disk))
	}
	if len(f.Network.Allow) > 0 || f.Network.Locked {
		network := "network: open"
		if len(f.Network.Allow) > 0 {
			network = "network: only " + strings.Join(f.Network.Allow, ", ")
		}
		if f.Network.Locked {
			network += ", locked"
		}
		plan = append(plan, network)
	}
	if len(f.Secrets) > 0 {
		plan = append(plan, "secrets: shared ones, plus "+strings.Join(f.Secrets, ", "))
	}
	plan = append(plan, claudePlan(f.Claude)...)
	if repo != "" {
		plan = append(plan, "clone: "+repo)
	}
	if f.Setup != "" {
		plan = append(plan, "setup, run in the VM:")
		for line := range strings.Lines(strings.TrimSpace(f.Setup)) {
			plan = append(plan, "  "+strings.TrimRight(line, "\n"))
		}
	}
	a.UI.Hint(plan...)
	if len(f.Without) > 0 {
		a.UI.Warn("without isn't needed any more: a project's VM gets only core and the packs it lists")
	}
	if len(missing) > 0 {
		a.UI.Warn("Not set up on this Mac, so the VM won't get them: %s", strings.Join(missing, ", "))
		a.UI.Hint("limabean secrets set NAME --vm " + f.Name)
	}
	if !yes {
		if !a.UI.Prompting {
			return errors.New("check the plan above, then run again with --yes to create it")
		}
		ok, err := a.UI.Confirm("Create "+f.Name+"?", true)
		if err != nil {
			return err
		}
		if !ok {
			a.UI.Info("Nothing created")
			return nil
		}
	}

	if f.CPUs > 0 {
		a.Cfg.CPUs = f.CPUs
	}
	if f.Memory > 0 {
		a.Cfg.Memory = fmt.Sprint(f.Memory)
	}
	if f.Disk > 0 {
		a.Cfg.Disk = f.Disk
	}
	o := createOptions{Name: f.Name, Env: firstNonEmpty(f.Env, "scratch")}
	if err := a.prepareVM(&o, vmChoices{secrets: own, exact: true, with: f.Packs, allow: f.Network.Allow, locked: f.Network.Locked}); err != nil {
		return err
	}
	a.prune(ctx)
	if err := a.create(ctx, o); err != nil {
		return err
	}
	inst, err := a.vm(ctx, f.Name)
	if err != nil {
		return err
	}
	clone := ""
	if repo != "" {
		if clone, err = a.cloneForRun(ctx, inst, repo); err != nil {
			return err
		}
	} else {
		a.UI.Warn("No repository to clone: the checkout has no origin, and the file names no repo")
	}
	a.upClaude(ctx, inst, f.Claude)
	if err := a.upSetup(inst, f, clone); err != nil {
		return err
	}
	a.upBox(f, clone)
	return nil
}

// projectRepo is what a project clones: repo in the file, or the
// checkout's origin; "" for neither.
func (a *App) projectRepo(ctx context.Context, f *project.File, dir string) string {
	if f.Repo != "" {
		return f.Repo
	}
	if origin, err := gitOrigin(ctx, dir); err == nil {
		return repoQuery(origin)
	}
	return ""
}

// upSetup runs the project's setup in its clone.
func (a *App) upSetup(inst lima.Instance, f *project.File, dir string) error {
	if f.Setup == "" {
		return nil
	}
	a.UI.Info("Running setup in %s", a.UI.Code(inst.Name))
	if err := a.attachSSH(inst, runScript(dir, []string{"bash", "-ec", f.Setup})); err != nil {
		a.UI.Hint("Look in the VM (limabean ssh "+inst.Name+"), fix the setup, then run it again:", "limabean up --setup")
		return fmt.Errorf("setup failed: %w", err)
	}
	a.UI.OK("Setup done")
	return nil
}

// upBox says how to reach a project's VM.
func (a *App) upBox(f *project.File, clone string) {
	lines := []string{a.UI.Code(f.Name) + " is ready", "", "ssh lima-" + f.Name + "     or: limabean ssh " + f.Name}
	if clone != "" {
		lines = append(lines, "limabean code "+f.Name+" "+clone)
	}
	for _, p := range f.Ports {
		lines = append(lines, fmt.Sprintf("http://localhost:%d", p))
	}
	a.UI.Box(lines...)
}

// projectSecrets finds the references for the secrets a project names:
// shared ones need nothing, others come from other VMs' files. It returns
// those, and the names that aren't set up anywhere.
func (a *App) projectSecrets(names []string) (own []secrets.Secret, missing []string) {
	shared, _ := secrets.Load(a.Cfg.Dir(), "")
	_, refs := a.otherVMSecrets("")
	for _, n := range names {
		if s, ok := refs[n]; ok {
			own = append(own, secrets.Secret{Name: n, Ref: s.Ref})
			continue
		}
		found := false
		for _, s := range shared {
			found = found || s.Name == n
		}
		if !found {
			missing = append(missing, n)
		}
	}
	return own, missing
}

// gitOrigin is a checkout's origin remote.
func gitOrigin(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").Output()
	return strings.TrimSpace(string(out)), err
}
