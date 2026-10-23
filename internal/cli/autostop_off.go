package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// newAutostopOffCmd builds `limabean autostop off`, which stops the
// background job that stops idle VMs.
func newAutostopOffCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Stop stopping idle VMs",
		Long: `Unload and remove the launchd agent that "autostop on" installed. Running
VMs keep running.`,
		Example: `  limabean autostop off`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = a.launchctl(cmd.Context(), "bootout", launchdTarget())
			if err := os.Remove(autostopPlist()); os.IsNotExist(err) {
				a.UI.Info("Autostop is already off")
				return nil
			} else if err != nil {
				return err
			}
			a.UI.OK("Autostop is off")
			return nil
		},
	}
}
