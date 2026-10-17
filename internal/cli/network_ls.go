package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNetworkLsCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "ls [name]",
		Aliases: []string{"list", "show"},
		Short:   "Show a VM's network policy",
		Long: `Show whether a VM's network is open or limited to an allowlist, the
domains it may reach, and whether it's locked. ntp.ubuntu.com is always
allowed alongside an allowlist, for the clock.`,
		Example: `  limabean network ls agent`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "show the network of")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.NetworkPolicy {
				a.UI.Info("%s's network is open, and it was created before network policies (rebuild it to use them)", name)
				return nil
			}
			p, err := a.loadPolicy(name)
			if err != nil {
				return err
			}
			a.UI.Section(name, p.summary())
			if p.open() {
				a.UI.OK("Open: it can reach any host")
			} else {
				a.UI.OK("Allowlist: only these domains and their subdomains")
				a.UI.Hint(strings.Join(p.Allow, "\n"))
			}
			if p.Locked {
				a.UI.OK("Locked: its user has no sudo or Docker")
			} else if !p.open() {
				a.UI.Warn("Not locked: anything in it can use sudo to turn the allowlist off")
				a.UI.Hint("limabean network lock " + name)
			}
			return nil
		},
	}
}
