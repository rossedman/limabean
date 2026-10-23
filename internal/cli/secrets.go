package cli

import "github.com/spf13/cobra"

// newSecretsCmd builds `limabean secrets`, the group of commands that manage
// the variables and secrets VMs get.
func newSecretsCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secrets",
		Aliases: []string{"secret"},
		Short:   "Manage the environment variables and secrets VMs get",
		Long: `Secrets are environment variables limabean installs in VMs: API keys,
tokens, or plain settings. On your Mac they're kept as references, never as
values, in two kinds of files with one NAME=reference per line:

  ~/.config/limabean/secrets.env         every VM
  ~/.config/limabean/secrets.<vm>.env    one VM; overrides the shared file

A reference says where the value comes from:

  keychain:<service>      an item in your macOS Keychain
  op://vault/item/field   a 1Password item, read with the op CLI
  env:<VAR>               a variable in the shell you run limabean in
  cmd:<command>           a command's output, run with sh on the Mac
  anything else           used as is (for settings, never for secrets)

"limabean secrets sync" looks each one up on the Mac and writes the values
to ~/.config/limabean/secrets.env (mode 600) in each VM, where every shell
loads them. New VMs get their secrets automatically.

Anything in a VM, Claude included, can read its secrets: prefer narrow,
short-lived credentials, and give a VM only what it needs.`,
		Example: `  # Paste a value now; it's saved in your Keychain
  limabean secrets set OPENAI_API_KEY

  # Point at 1Password instead, for one VM only
  limabean secrets set NPM_TOKEN op://Work/npm/token --vm dev

  # See what each VM gets, then push changes to running VMs
  limabean secrets ls
  limabean secrets sync`,
	}
	cmd.AddCommand(newSecretsLsCmd(a), newSecretsSetCmd(a), newSecretsRmCmd(a), newSecretsEditCmd(a), newSecretsSyncCmd(a))
	return cmd
}
