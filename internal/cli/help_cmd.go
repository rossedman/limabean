package cli

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// cmdSchema is one command in help --json.
type cmdSchema struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Usage    string       `json:"usage"`
	Short    string       `json:"short"`
	Long     string       `json:"long,omitempty"`
	Aliases  []string     `json:"aliases,omitempty"`
	Flags    []flagSchema `json:"flags"`
	Commands []cmdSchema  `json:"commands,omitempty"`
}

// flagSchema is one flag in help --json.
type flagSchema struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Usage     string `json:"usage"`
	Global    bool   `json:"global,omitempty"` // a persistent flag every command takes
}

// newHelpCmd builds `limabean help`, which shows help for any command.
func newHelpCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Show help for any command",
		Long: `Show help for limabean or one of its commands.

With --json it prints the command tree instead: every command below the one
named, with its usage, description, aliases and flags, for scripts and
agents. Hidden commands and flags (old names kept working) aren't listed.`,
		Example: `  limabean help up
  limabean help --json | jq -r '.commands[].name'
  limabean help network --json`,
		ValidArgsFunction: func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			cmd, _, err := c.Root().Find(args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			for _, sub := range cmd.Commands() {
				if sub.IsAvailableCommand() {
					names = append(names, sub.Name())
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(c *cobra.Command, args []string) error {
			target, _, err := c.Root().Find(args)
			if err != nil || target == nil {
				return usageErr("unknown help topic %q: run \"limabean help\" for the commands", strings.Join(args, " "))
			}
			if !asJSON {
				target.InitDefaultHelpFlag()
				return target.Help()
			}
			enc := json.NewEncoder(a.UI.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(schemaOf(target))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the command tree as JSON, for scripts")
	return cmd
}

// schemaOf describes c and the commands below it for help --json.
func schemaOf(c *cobra.Command) cmdSchema {
	s := cmdSchema{
		Name: c.Name(), Path: c.CommandPath(), Usage: c.UseLine(), Short: c.Short, Long: c.Long,
		Aliases: c.Aliases, Flags: []flagSchema{},
	}
	add := func(global bool) func(*pflag.Flag) {
		return func(f *pflag.Flag) {
			if f.Hidden || f.Name == "help" {
				return
			}
			s.Flags = append(s.Flags, flagSchema{
				Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type(),
				Default: f.DefValue, Usage: f.Usage, Global: global,
			})
		}
	}
	c.LocalNonPersistentFlags().VisitAll(add(false))
	// Global flags are listed once, on the command that defines them.
	c.PersistentFlags().VisitAll(add(!c.HasParent()))
	for _, sub := range c.Commands() {
		if sub.IsAvailableCommand() {
			s.Commands = append(s.Commands, schemaOf(sub))
		}
	}
	return s
}
