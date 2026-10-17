package cli

import "github.com/spf13/cobra"

func newSecretsSyncCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync [name]",
		Short: "Install secrets in running VMs",
		Long: `Look up every secret on the Mac and write the values into VMs, replacing
what they had: secrets that were removed disappear too.

Without a name, every running VM is updated and stopped ones are listed
(they keep their last secrets until synced). With a name, that VM is
updated, and started first if it's stopped.

A secret that can't be looked up (a missing Keychain item, a locked
1Password, an unset variable) is reported and left out; the rest are still
installed. Run it after rotating a token or editing a secrets file.`,
		Example: `  limabean secrets sync

  # Just dev, starting it if needed
  limabean secrets sync dev`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return a.syncSecrets(cmd.Context(), name)
		},
	}
}
