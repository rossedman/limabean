package cli

import (
	"github.com/spf13/cobra"
)

// newStartCmd builds `limabean start`, the older way to start a stopped VM.
func newStartCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:    "start [name]",
		Hidden: true, // "limabean up <name>"; start stays for scripts
		PreRun: a.oldCommand("limabean up <name>"),
		Short:  "Start a stopped VM (limabean up <name> does this now)",
		Long: `Boot a stopped VM and add its Claude Desktop connection back ("stop"
removes it, since Claude Desktop shows an error for a VM it can't reach).
"limabean ssh" and "limabean get" also start a stopped VM; use start when
you want it running without opening a shell, e.g. for Claude Desktop.

Leave out the name to pick a VM from a list.`,
		Example: `  limabean start dev

  # Pick the VM from a list
  limabean start`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "start")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if inst.Running() {
				a.claudeAdd(name)
				a.UI.Info("%s is already running", name)
				return nil
			}
			if _, err := a.ensureRunning(ctx, inst); err != nil {
				return err
			}
			a.UI.OK("Started %s", a.UI.Code(name))
			return nil
		},
	}
}
