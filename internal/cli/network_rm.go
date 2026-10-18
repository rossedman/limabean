package cli

import (
	"errors"
	"slices"

	"github.com/spf13/cobra"
)

func newNetworkRmCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name> <domain|preset>...",
		Aliases: []string{"remove"},
		Short:   "Stop a VM reaching some domains",
		Long: `Remove domains or presets from a VM's allowlist. Removing the last one
doesn't open the network: use "limabean network open" for that.`,
		Example: `  limabean network rm agent python`,
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			domains, err := expandAllow(args[1:])
			if err != nil {
				return err
			}
			return a.changePolicy(cmd.Context(), args[0], func(p *netPolicy) error {
				keep := slices.DeleteFunc(slices.Clone(p.Allow), func(d string) bool { return slices.Contains(domains, d) })
				if len(keep) == len(p.Allow) {
					return errors.New("none of those are in the allowlist (see: limabean network ls " + args[0] + ")")
				}
				if len(keep) == 0 {
					return errors.New("that would empty the allowlist, which opens the network; use: limabean network open " + args[0])
				}
				p.Allow = keep
				return nil
			})
		},
	}
}
