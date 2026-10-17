package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newAutostopStatusCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether idle VMs are stopped automatically",
		Long: `Show whether the autostop agent is installed and loaded, and after how
long an unused VM is stopped. To see what it would do right now, run
"limabean stop --idle <after> --dry-run".`,
		Example: `  limabean autostop status`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plist, err := os.ReadFile(autostopPlist())
			if os.IsNotExist(err) {
				a.UI.Info("Autostop is off")
				a.UI.Hint("Turn it on with: limabean autostop on")
				return nil
			} else if err != nil {
				return err
			}
			after := autostopAfter(plist)
			if _, err := a.launchctl(cmd.Context(), "print", launchdTarget()); err != nil {
				a.UI.Warn("Autostop is installed but not loaded, so it isn't running")
				a.UI.Hint("Load it again with: limabean autostop on --after " + after)
				return errSilent
			}
			a.UI.OK("VMs nobody has used for %s are stopped (checked every %s)", after, shortDuration(autostopInterval))
			a.UI.Hint("Log: "+shortPath(autostopLog()), "Preview: limabean stop --idle "+after+" --dry-run")
			return nil
		},
	}
}
