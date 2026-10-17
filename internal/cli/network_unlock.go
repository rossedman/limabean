package cli

import (
	"github.com/spf13/cobra"
)

func newNetworkUnlockCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "unlock <name>",
		Short: "Give a VM's user sudo and Docker back",
		Long: `Unlock a VM: its user gets passwordless sudo back at once, and Docker on
the next boot (limabean stop and start). Its allowlist, if it has one, stays,
but anything in the VM can now turn it off.`,
		Example: `  limabean network unlock agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.changePolicy(cmd.Context(), args[0], func(p *netPolicy) error {
				p.Locked = false
				return nil
			})
		},
	}
}
