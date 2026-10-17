package cli

import "github.com/spf13/cobra"

func newClaudeCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Keep VMs listed as SSH connections in Claude Desktop",
		Long: `Claude Desktop reads SSH connections from the "sshConfigs" list in
~/.claude/settings.json. limabean adds an entry for every VM it creates
("dev (limabean)", host lima-dev) and removes it when the VM is deleted,
touching only entries whose id starts with "limabean-". Other settings are
kept, and a file that isn't valid JSON is left alone.

On first connect, Claude Desktop installs its own copy of Claude Code in the
VM. Set CLAUDE_DESKTOP="no" in the config to turn this integration off.`,
		Example: `  # Re-register every VM and drop deleted ones
  limabean claude sync`,
	}
	cmd.AddCommand(newClaudeSyncCmd(a))
	return cmd
}
