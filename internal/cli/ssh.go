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

Inside a project's checkout, the project's VM is the default, and the shell
or command runs in its clone of the repository rather than the home
directory. Start with -- to give a command without naming the VM.

This is the same as "ssh lima-<name>", but works even if your ~/.ssh/config
doesn't include Lima's host entries.`,
		Example: `  # A shell in dev
  limabean ssh dev

  # Pick the VM from a list
  limabean ssh

  # Run one command
  limabean ssh dev docker ps -a
  limabean ssh dev -- ls -la ~/code

  # In a project's checkout: its tests, in the VM's clone
  limabean ssh -- make test`,
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			nameArgs, command := args, []string(nil)
			if cmd.ArgsLenAtDash() == 0 {
				// "ssh -- make test": no name, all command.
				nameArgs, command = nil, args
			} else if len(args) > 1 {
				command = args[1:]
				// Flags aren't parsed after the name, so a -- is still here.
				if command[0] == "--" {
					command = command[1:]
				}
			}
			name, err := a.projectVMArg(ctx, nameArgs, "open")
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
			if dir := a.projectCloneHere(ctx, inst); dir != "" {
				return a.execSSHIn(inst, dir, command...)
			}
			return a.execSSH(inst, command...)
		},
	}
	// Flags after the VM name belong to the remote command.
	cmd.Flags().SetInterspersed(false)
	return cmd
}
