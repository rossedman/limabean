package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// newGetCmd builds `limabean get`, which clones a repository into a VM with
// ghq.
func newGetCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [name] <repo> [ghq flags...]",
		Short: "Clone a repository into a VM with ghq",
		Long: `Clone a repository into a VM's ~/code with ghq, which lays it out as
~/code/<host>/<owner>/<repo>. A stopped VM is started first.

With only a repository, you pick the VM from a list. Flags after the
repository are passed to "ghq get": -p clones over SSH, which private
repositories need unless a GitHub token is installed (limabean github login).`,
		Example: `  # Into dev: ~/code/github.com/rossedman/limabean
  limabean get dev rossedman/limabean

  # A full URL works too
  limabean get dev https://github.com/charmbracelet/gum

  # A private repository over SSH
  limabean get dev -p acme/private-repo

  # Pick the VM from a list
  limabean get acme/private-repo -p`,
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var name string
			var ghqArgs []string
			switch {
			case len(args) >= 2:
				name, ghqArgs = args[0], args[1:]
			case len(args) == 1:
				picked, err := a.projectVMArg(ctx, nil, "clone "+args[0]+" into")
				if err != nil {
					return err
				}
				name, ghqArgs = picked, args
			default:
				return errors.New("name a repository, e.g. limabean get dev rossedman/limabean")
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if inst, err = a.ensureRunning(ctx, inst); err != nil {
				return err
			}
			return a.attachSSH(inst, append([]string{"ghq", "get"}, flagsFirst(ghqArgs)...)...)
		},
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// flagsFirst moves flags ahead of the repository, where ghq expects them.
func flagsFirst(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}
