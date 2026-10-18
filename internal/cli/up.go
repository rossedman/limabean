package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/secrets"
)

func newUpCmd(a *App) *cobra.Command {
	var file string
	var yes bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create or start the VM a repository's .limabean.yaml describes",
		Long: `Bring up the VM for the project you're in. A repository describes the VM it
needs in .limabean.yaml (or .limabean.yml), here or in a parent directory.
up creates that VM, clones the repository into it and runs its setup, or
starts it if it already exists. Changes to the file reach a VM made
before them when it's made again: limabean rm <name> && limabean up.

  # .limabean.yaml
  name: myapp                 # default: the directory's name
  env: scratch                # environment template
  packs: [rust, claude]       # added to the default packs
  without: [kubernetes]       # default packs to leave out
  cpus: 6
  memory: 12                  # GiB
  disk: 30                    # GiB of system disk
  network:                    # see: limabean network --help
    allow: [github, npm]
    locked: true
  secrets: [OPENAI_API_KEY]   # names only; values stay on your Mac
  ports: [3000]               # shown as http://localhost:<port> after up
  claude:                     # for Claude Code in the VM
    marketplaces:             # name: GitHub owner/repo, or an https git URL
      guardrails: rossedman/guardrails
    plugins: [guardrails@guardrails]
    skills: [anthropics/skills/skills/pdf]   # owner/repo/path[@branch or tag]
  setup: |                    # run once in the clone, with bash -e
    make deps

The repository cloned is the checkout's origin (or repo: in the file).
Secrets are names only: each must already be set up on your Mac (limabean
secrets ls), and shared secrets are included as always.

Claude plugins go into the VM's ~/.claude/settings.json, and Claude Code
fetches them when a session starts; skills are copied into ~/.claude/skills.
Both are fetched inside the VM, so one with a network allowlist needs
github (or the marketplace's host).

The file comes with the repository, so up shows what it asks for and
waits for you to confirm before creating anything; --yes skips that, for
files you trust.`,
		Example: `  cd ~/src/myapp && limabean up

  # From a script, for a file you trust
  limabean up --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			path := file
			if path == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				if path, err = project.Find(wd); err != nil {
					return err
				}
			}
			f, err := project.Load(path)
			if err != nil {
				return err
			}
			if inst, ok, err := a.Lima.Get(ctx, f.Name); err != nil {
				return err
			} else if ok {
				if _, err := a.ensureRunning(ctx, inst); err != nil {
					return err
				}
				a.UI.OK("%s is up", a.UI.Code(f.Name))
				a.upBox(f, "")
				return nil
			}
			return a.upCreate(ctx, f, filepath.Dir(path), yes)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "the project file (default: .limabean.yaml here or in a parent directory)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "create the VM without asking")
	return cmd
}

// upCreate creates a project's VM, clones the project and runs its setup.
func (a *App) upCreate(ctx context.Context, f *project.File, dir string, yes bool) error {
	repo := f.Repo
	if repo == "" {
		if origin, err := gitOrigin(ctx, dir); err == nil {
			repo = repoQuery(origin)
		}
	}
	own, missing := a.projectSecrets(f.Secrets)
	a.UI.Section(f.Name, "from "+shortPath(filepath.Join(dir, project.FileName)))
	plan := []string{"environment: " + firstNonEmpty(f.Env, "scratch")}
	if len(f.Packs) > 0 || len(f.Without) > 0 {
		packs := "packs: the defaults"
		if len(f.Packs) > 0 {
			packs += ", plus " + strings.Join(f.Packs, ", ")
		}
		if len(f.Without) > 0 {
			packs += ", without " + strings.Join(f.Without, ", ")
		}
		plan = append(plan, packs)
	}
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
	if err := a.prepareVM(&o, vmChoices{secrets: own, with: f.Packs, without: f.Without, allow: f.Network.Allow, locked: f.Network.Locked}); err != nil {
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

// upSetup runs the project's setup in its clone.
func (a *App) upSetup(inst lima.Instance, f *project.File, dir string) error {
	if f.Setup == "" {
		return nil
	}
	a.UI.Info("Running setup in %s", a.UI.Code(inst.Name))
	if err := a.attachSSH(inst, runScript(dir, []string{"bash", "-ec", f.Setup})); err != nil {
		a.UI.Hint("Fix it in the VM (limabean ssh "+inst.Name+"), or change the setup and start over:", "limabean rm "+inst.Name+" && limabean up")
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
