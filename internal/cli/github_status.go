package cli

import (
	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/ui"

	"github.com/rossedman/limabean/internal/github"
)

func newGitHubStatusCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the GitHub token's account and expiry, and which VMs have it",
		Long: `Check the saved GitHub token with GitHub, showing the account it belongs
to and when it expires, then list each VM: whether it has the token
installed, or is stopped.

A missing token in a running VM is fixed with "limabean github sync".`,
		Example: `  limabean github status`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// The report is the result, so it goes to stdout.
			defer func(u *ui.UI) { a.UI = u }(a.UI)
			a.UI = a.UI.Report()
			ctx := cmd.Context()
			token, err := a.Keychain.Get(a.githubService())
			if err != nil {
				a.UI.Warn("No GitHub token saved; gh in VMs isn't logged in")
				a.UI.Hint("limabean github login")
				return nil
			}
			if info, err := a.GitHub.Check(ctx, token); err != nil {
				a.UI.Bad("The saved token doesn't work: %v", err)
				a.UI.Hint("limabean github login")
			} else {
				a.UI.OK("Token for %s, expires: %s", a.UI.Code(info.Login), info.Expires)
			}
			list, err := a.vms(ctx)
			if err != nil {
				return err
			}
			for _, inst := range list {
				switch {
				case !inst.Running():
					a.UI.Hint(padRight(inst.Name, 16) + "stopped")
				case a.vmHasSecret(ctx, inst, github.SecretName):
					a.UI.OK("%sinstalled", padRight(inst.Name, 16))
				default:
					a.UI.Warn("%smissing (limabean github sync)", padRight(inst.Name, 16))
				}
			}
			return nil
		},
	}
}
