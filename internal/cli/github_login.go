package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/github"
	"github.com/rossedman/limabean/internal/secrets"
)

func newGitHubLoginCmd(a *App) *cobra.Command {
	var withToken, noSync bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save a GitHub token and install it in VMs",
		Long: `Check a GitHub token with GitHub, save it to your Keychain, make it the
GITHUB_TOKEN secret for every VM, and install it in running VMs. New VMs
get it automatically.

In a terminal you're asked for the token at a hidden prompt. With
--with-token (like "gh auth login --with-token") it's read from stdin, for
scripts and password managers. A token that isn't fine-grained is accepted
with a warning, since classic and gh tokens usually have broad scopes.

Run it again with a new token when the old one expires.`,
		Example: `  # Paste a token
  limabean github login

  # From 1Password
  op read "op://Private/GitHub limabean/token" | limabean github login --with-token

  # Reuse your host gh login
  gh auth token --hostname github.com | limabean github login --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var token string
			var err error
			switch {
			case withToken:
				if a.UI.Prompting {
					return errors.New("--with-token reads the token from stdin; pipe it in: … | limabean github login --with-token")
				}
				token, err = readLine(cmd)
			case a.UI.Prompting:
				a.UI.Box("Create a fine-grained token limited to the repositories you'll use in VMs:",
					a.UI.Code("https://github.com/settings/personal-access-tokens/new"), "",
					"Give it an expiry and only the permissions you need, for example",
					"Contents and Pull requests: read and write. It's kept in your Keychain.")
				token, err = a.UI.Password("GitHub token", "github_pat_…")
			default:
				token, err = readLine(cmd)
			}
			if err != nil {
				return err
			}
			token = strings.TrimSpace(token)
			if token == "" {
				return errors.New("no token given")
			}
			var info github.TokenInfo
			if err := a.UI.Spin(cmd.Context(), "Checking the token with GitHub", func(ctx contextT) error {
				var err error
				info, err = a.GitHub.Check(ctx, token)
				return err
			}); err != nil {
				return err
			}
			a.UI.OK("Token works for %s, expires: %s", a.UI.Code(info.Login), info.Expires)
			if !github.FineGrained(token) {
				a.UI.Warn("This isn't a fine-grained token (they start with github_pat_).")
				a.UI.Hint("Classic and gh tokens usually have broad scopes, and anything in a VM can use it.")
			}
			if err := a.Keychain.Set(a.githubService(), token); err != nil {
				return fmt.Errorf("saving the token to the Keychain: %w", err)
			}
			if err := secrets.Put(secrets.SharedFile(a.Cfg.Dir()), github.SecretName, a.githubRef()); err != nil {
				return err
			}
			a.UI.OK("Saved to your Keychain as %s, used for %s in every VM", a.githubService(), github.SecretName)
			if noSync {
				return nil
			}
			a.UI.Section("Installing it in running VMs", "")
			return a.syncSecrets(cmd.Context(), "")
		},
	}
	cmd.Flags().BoolVar(&withToken, "with-token", false, "read the token from stdin")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "don't install it in running VMs yet")
	return cmd
}
