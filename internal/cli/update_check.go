package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newUpdateCheckCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:    "update-check",
		Hidden: true, // run in the background, once a day, by any command in a terminal
		Short:  "Check GitHub for a newer limabean release",
		Long: `Ask GitHub for limabean's latest release and remember it, so the next
command in a terminal can say when there's a newer one. Commands start this
in the background once a day; UPDATE_CHECK=no turns it off.`,
		Example: `  limabean update-check`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tag, err := a.GitHub.LatestRelease(cmd.Context(), limabeanRepo)
			if err != nil {
				return err
			}
			if !releaseRE.MatchString(tag) {
				return usageErr("GitHub's latest release, %q, isn't a version", tag)
			}
			return a.saveUpdateState(updateState{Checked: time.Now(), Latest: tag})
		},
	}
}
