package cli

import (
	"github.com/spf13/cobra"
)

func newNetworkOpenCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "open <name>",
		Short: "Let a VM reach any host again",
		Long: `Remove a VM's allowlist, so it can reach any host, and stop recording its
lookups if it was learning. A locked VM stays
locked: use "limabean network unlock" too to give its user sudo back.`,
		Example: `  limabean network open agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.changePolicy(cmd.Context(), args[0], func(p *netPolicy) error {
				p.Allow, p.Learn = nil, false
				return nil
			})
		},
	}
}
