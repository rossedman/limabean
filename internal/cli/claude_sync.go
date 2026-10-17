package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newClaudeSyncCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Register every VM in Claude Desktop and remove deleted ones",
		Long: `Make Claude Desktop's SSH connections match your VMs: add an entry for each
VM built from limabean's templates, and remove entries for VMs that no
longer exist. "new", "rm" and "ls" already do this as they go; sync is for
VMs created before the integration, or after editing settings.json by hand.

Restart Claude Desktop if a VM doesn't show up in its environment dropdown.`,
		Example: `  limabean claude sync`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.Cfg.ClaudeDesktop {
				return errors.New(`CLAUDE_DESKTOP is "no" in the config`)
			}
			ctx := cmd.Context()
			a.prune(ctx)
			list, err := a.vms(ctx)
			if err != nil {
				return err
			}
			for _, inst := range list {
				if err := a.Claude.Add(inst.Name); err != nil {
					return err
				}
				a.UI.OK("%s → ssh lima-%s", a.UI.Code(inst.Name+" (limabean)"), inst.Name)
			}
			if len(list) == 0 {
				a.UI.Info("No VMs to register.")
			}
			return nil
		},
	}
}
