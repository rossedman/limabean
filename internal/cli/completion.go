package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCmd(_ *App) *cobra.Command {
	return &cobra.Command{
		Use:   "completion <bash|zsh|fish|powershell>",
		Short: "Print a shell completion script",
		Long: `Print a script that teaches your shell to tab-complete limabean's commands,
flags and VM names.

Load it once to try it, or save it where your shell loads completions from
to keep it.`,
		Example: `  # zsh: try it now
  source <(limabean completion zsh)

  # zsh: keep it (any directory on your fpath works)
  limabean completion zsh > "${fpath[1]}/_limabean"

  # bash with bash-completion installed
  limabean completion bash > $(brew --prefix)/etc/bash_completion.d/limabean

  # fish
  limabean completion fish > ~/.config/fish/completions/limabean.fish`,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		DisableFlagsInUseLine: true,
		// Completion scripts don't need the config or a terminal.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			root, out := cmd.Root(), cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			}
			return fmt.Errorf("unsupported shell %q (bash, zsh, fish or powershell)", args[0])
		},
	}
}
