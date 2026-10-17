package cli

import "github.com/spf13/cobra"

func newGitHubSyncCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Install the GitHub token in every running VM",
		Long: `Install the GitHub token, along with every other secret, in each running
VM. It's the same as "limabean secrets sync": use it after starting a VM
that was stopped when the token changed.`,
		Example: `  limabean github sync`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := a.Keychain.Get(a.githubService()); err != nil {
				a.UI.Warn("No GitHub token saved (limabean github login); syncing the other secrets")
			}
			return a.syncSecrets(cmd.Context(), "")
		},
	}
}
