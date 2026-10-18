package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// hiddenAlias adds a hidden bool flag that sets the same variable as target,
// so an old or familiar name keeps working without showing up in help.
func hiddenAlias(fs *pflag.FlagSet, p *bool, name, short, target string) {
	fs.BoolVarP(p, name, short, false, "same as --"+target)
	_ = fs.MarkHidden(name)
}

// oldCommand is the PreRun of a hidden command kept for scripts: a note on
// stderr naming the command that replaced it. MCP's own calls and
// "stop --idle" from autostop agents installed before down don't get one.
func (a *App) oldCommand(now string) func(*cobra.Command, []string) {
	return func(c *cobra.Command, _ []string) {
		if a.mcpMode || c.Flags().Lookup("idle") != nil && c.Flags().Changed("idle") {
			return
		}
		a.UI.Info("Note: %q is now %q; %s still works, but isn't in help.", "limabean "+c.CalledAs(), now, c.CalledAs())
	}
}
