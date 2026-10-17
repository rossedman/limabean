package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newStopCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running VM",
		Long: `Shut a VM down. Its disk, and everything in ~/code, is kept; start it again
with "limabean ssh <name>" or "limactl start <name>".

Leave out the name to pick a VM from a list.`,
		Example: `  limabean stop dev

  # Pick the VM from a list
  limabean stop`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "stop")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.Running() {
				a.UI.Info("%s is already stopped", name)
				return nil
			}
			if err := a.limaStep(ctx, "Stopping "+name, func(ctx context.Context) ([]byte, error) {
				return a.Lima.Stop(ctx, name)
			}); err != nil {
				return err
			}
			a.UI.OK("Stopped %s", a.UI.Code(name))
			return nil
		},
	}
}
