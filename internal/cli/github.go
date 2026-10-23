package cli

import "github.com/spf13/cobra"

// newGitHubCmd builds `limabean github`, the group of commands that give VMs
// a GitHub token.
func newGitHubCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Give VMs a GitHub token for gh, the API and https pushes",
		Long: `Manage the GitHub token VMs use. It's stored in your macOS Keychain
(limabean-github-token) and installed in VMs as the GITHUB_TOKEN secret, so
it's one of your secrets, with a few shortcuts here.

With a token, git in VMs talks to GitHub over https with it, including for
SSH-style remotes, so pushing works in Claude Desktop sessions, which have no
forwarded ssh-agent. Without one, git pushes over SSH with your agent.

Use a fine-grained token limited to the repositories you'll work on, with an
expiry and only the permissions you need (Contents and Pull requests: read
and write is typical). Anything in a VM, Claude included, can use it.`,
		Example: `  # Paste a fine-grained token
  limabean github login

  # Reuse your gh login (broader scopes; limabean warns)
  gh auth token --hostname github.com | limabean github login --with-token

  limabean github status`,
	}
	cmd.AddCommand(newGitHubLoginCmd(a), newGitHubStatusCmd(a), newGitHubSyncCmd(a), newGitHubLogoutCmd(a))
	return cmd
}
