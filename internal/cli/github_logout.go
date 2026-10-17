package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/github"
	"github.com/rossedman/limabean/internal/secrets"
)

func newGitHubLogoutCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the GitHub token from the Keychain and VMs",
		Long: `Delete the GitHub token from your Keychain, remove the GITHUB_TOKEN secret
that points at it, and update running VMs, so git there goes back to
pushing over SSH.

This doesn't revoke the token: do that on GitHub too.`,
		Example: `  limabean github logout`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.Keychain.Delete(a.githubService()); err != nil {
				return errors.New("no GitHub token saved")
			}
			a.UI.OK("Removed the token from your Keychain")
			shared := secrets.SharedFile(a.Cfg.Dir())
			list, _ := secrets.LoadFile(shared)
			for _, s := range list {
				if s.Name == github.SecretName && s.Ref == a.githubRef() {
					if err := secrets.Put(shared, github.SecretName, ""); err != nil {
						return err
					}
				}
			}
			a.UI.Section("Removing it from running VMs", "")
			err := a.syncSecrets(cmd.Context(), "")
			a.UI.Hint("Revoke it on GitHub too: https://github.com/settings/personal-access-tokens")
			return err
		},
	}
}
