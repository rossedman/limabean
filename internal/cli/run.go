package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
)

func newRunCmd(a *App) *cobra.Command {
	var (
		name, env, repo                        string
		remove, locked                         bool
		ttl                                    time.Duration
		secretFlags, allowFlags, with, without []string
	)
	cmd := &cobra.Command{
		Use:   "run [flags] -- <command>...",
		Short: "Run a command in a new VM, and delete it after",
		Long: `Create a VM, run one command in it, and with --rm delete the VM when the
command ends, whether it worked or not. limabean exits with the command's
exit status.

--repo clones a repository first and runs the command in it. The VM gets
your shared secrets, the PACKS setting's packs (none besides core, unless
you set it) and an open network, unless flags say otherwise, as for
"limabean up <name>". Without --rm the VM is kept, named
run-<random> unless you name it; --ttl deletes it later.

A single argument is run by the shell as it is, so pipes and && work;
several arguments are quoted one by one.`,
		Example: `  # A repository's tests in a clean VM, then delete it
  limabean run --rm --repo rossedman/limabean -- go test ./...

  # Claude Code on a task, in a locked sandbox that can only reach GitHub and Anthropic
  limabean run --rm --with claude --allow github --allow claude --locked \
    --repo me/app -- 'claude -p "make the tests pass" && git push'

  # Keep the VM for two hours afterwards
  limabean run --ttl 2h --name spike -- 'make bench | tee bench.txt'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if name == "" {
				name = "run-" + randomHex(2)
			}
			if err := validVMName(name); err != nil {
				return usageErr("VM name %q: %w", name, err)
			}
			if _, exists, err := a.Lima.Get(ctx, name); err != nil {
				return err
			} else if exists {
				return conflictErr("a VM named %q already exists", name)
			}
			flagSecrets, err := a.parseSecretFlags(secretFlags)
			if err != nil {
				return err
			}
			o := createOptions{Name: name, Env: env}
			if err := a.prepareVM(&o, vmChoices{secrets: flagSecrets, with: with, without: without, allow: allowFlags, locked: locked}); err != nil {
				return err
			}
			a.prune(ctx)
			if err := a.create(ctx, o); err != nil {
				return err
			}
			if remove {
				defer a.deleteNow(context.WithoutCancel(ctx), name)
			} else if err := a.setTTL(ctx, name, ttl); err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			dir := ""
			if repo != "" {
				if dir, err = a.cloneForRun(ctx, inst, repo); err != nil {
					return err
				}
			}
			a.UI.Info("Running in %s: %s", a.UI.Code(name), strings.Join(args, " "))
			err = a.attachSSH(inst, runScript(dir, args))
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				a.UI.Warn("The command exited with status %d", exit.ExitCode())
				err = exitCode(exit.ExitCode())
			}
			if err == nil && !remove {
				a.UI.Hint("The VM is kept: limabean ssh " + name + "   (limabean rm " + name + " when done)")
			}
			return err
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "name the VM (default run-<random>)")
	f.StringVar(&env, "env", "scratch", "environment template")
	f.StringVar(&repo, "repo", "", "clone this repository first and run the command in it, e.g. owner/repo")
	f.BoolVar(&remove, "rm", false, "delete the VM when the command ends")
	f.DurationVar(&ttl, "ttl", 0, "without --rm: delete the VM once it's this old, e.g. 2h")
	f.StringArrayVar(&secretFlags, "secret", nil, "give the VM a secret: NAME=reference (repeatable)")
	f.StringSliceVar(&with, "with", nil, "give the VM these packs, e.g. go,claude")
	f.StringSliceVar(&without, "without", nil, "leave these packs out")
	f.StringArrayVar(&allowFlags, "allow", nil, "only let the VM reach this domain or preset (repeatable)")
	f.BoolVar(&locked, "locked", false, "take sudo and Docker away from the VM's user, so its allowlist holds")
	cmd.MarkFlagsMutuallyExclusive("rm", "ttl")
	f.SetInterspersed(false)
	return cmd
}

// cloneForRun clones repo into a VM and returns where it is.
func (a *App) cloneForRun(ctx context.Context, inst lima.Instance, repo string) (string, error) {
	if strings.HasPrefix(repo, "-") {
		return "", fmt.Errorf("%q isn't a repository", repo)
	}
	if err := a.limaStep(ctx, "Cloning "+repo, func(ctx context.Context) ([]byte, error) {
		return a.Lima.Run(ctx, inst, nil, "ghq get "+secrets.ShellQuote(repo))
	}); err != nil {
		return "", err
	}
	return a.codeDir(ctx, inst, repo)
}

// deleteNow deletes a VM made by run --rm, without checks.
func (a *App) deleteNow(ctx context.Context, name string) {
	if err := a.limaStep(ctx, "Deleting "+name, func(ctx context.Context) ([]byte, error) {
		return a.Lima.Delete(ctx, name)
	}); err != nil {
		a.UI.Warn("%v", err)
		a.UI.Hint("limabean rm " + name + " --force")
		return
	}
	a.forget(ctx, name)
	a.UI.OK("Deleted %s", a.UI.Code(name))
}

// runScript is the remote command for run: one argument as the shell's
// own, several quoted one by one, in dir if it's set.
func runScript(dir string, args []string) string {
	script := args[0]
	if len(args) > 1 {
		quoted := make([]string, len(args))
		for i, arg := range args {
			quoted[i] = secrets.ShellQuote(arg)
		}
		script = strings.Join(quoted, " ")
	}
	if dir != "" {
		script = "cd " + secrets.ShellQuote(dir) + " && " + script
	}
	return script
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
