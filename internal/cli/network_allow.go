package cli

import (
	"github.com/spf13/cobra"
)

func newNetworkAllowCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "allow <name> <domain|preset>...",
		Short: "Let a VM reach more domains",
		Long: `Add domains or presets to a VM's allowlist. A VM with an open network gets
an allowlist of just these, so everything else is blocked from then on.

A domain allows its subdomains: github.com covers api.github.com. Presets:

` + presetHelp(),
		Example: `  limabean network allow agent github python
  limabean network allow agent registry.example.com`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			domains, err := expandAllow(args[1:])
			if err != nil {
				return err
			}
			return a.changePolicy(cmd.Context(), args[0], func(p *netPolicy) error {
				p.Allow = append(p.Allow, domains...)
				return nil
			})
		},
	}
}
