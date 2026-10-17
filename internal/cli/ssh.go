package cli

import (
	"github.com/spf13/cobra"
)

func newSSHCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh [name] [command...]",
		Aliases: []string{"shell"},
		Short:   "Open a shell in a VM, or run a command in it",
		Long: `Open an interactive shell in a VM, or run a single command in it. A stopped
VM is started first.

Leave out the name to pick a VM from a list. Everything after the name is
the command to run; flags after the name belong to that command, not to
limabean.

This is the same as "ssh lima-<name>", but works even if your ~/.ssh/config
doesn't include Lima's host entries.`,
		Example: `  # A shell in dev
  limabean ssh dev

  # Pick the VM from a list
  limabean ssh

  # Run one command
  limabean ssh dev docker ps -a
  limabean ssh dev -- ls -la ~/code`,
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "open")
			if err != nil {
				return err
			}
			var command []string
			if len(args) > 1 {
				command = args[1:]
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if inst, err = a.ensureRunning(ctx, inst); err != nil {
				return err
			}
			return a.execSSH(inst, command...)
		},
	}
	// Flags after the VM name belong to the remote command.
	cmd.Flags().SetInterspersed(false)
	return cmd
}
