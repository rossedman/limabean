package cli

import (
	"github.com/spf13/cobra"
)

func newNetworkLockCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "lock <name>",
		Short: "Take sudo and Docker away from a VM's user",
		Long: `Lock a VM: its user loses passwordless sudo, the sudo and docker groups,
and the Docker socket, so nothing running as that user (an agent
included) can become root and turn its allowlist off. The lock is
re-applied on every boot.

Lock before handing a VM to an agent: a root process started before the
lock keeps running. "limabean network unlock" undoes it.`,
		Example: `  limabean network lock agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.changePolicy(cmd.Context(), args[0], func(p *netPolicy) error {
				p.Locked = true
				return nil
			})
		},
	}
}
