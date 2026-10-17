package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNetworkLearnCmd(a *App) *cobra.Command {
	var done bool
	cmd := &cobra.Command{
		Use:   "learn <name>",
		Short: "Record what a VM reaches, to build its allowlist",
		Long: `Find out which domains a VM's work needs. While it learns, the VM can reach
any host, and every DNS lookup it or its containers make is recorded: run
the task, the tests or the install the way an agent would.

"limabean network log" shows what was looked up. --done ends learning and
allows the domains seen (a.b.example.com is allowed as example.com), on
top of any already on the allowlist. The VM is limited to them from then
on, so lock it too if an agent will use it.

Lookups are recorded until the VM restarts.`,
		Example: `  limabean network learn agent
  limabean ssh agent make test
  limabean network log agent
  limabean network learn agent --done
  limabean network lock agent`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			if !done {
				if err := a.changePolicy(ctx, name, func(p *netPolicy) error {
					p.Learn = true
					return nil
				}); err != nil {
					return err
				}
				a.UI.Hint("Now use "+name+" for the task it's for.",
					"See what it looks up: limabean network log "+name,
					"Then limit it to that: limabean network learn "+name+" --done")
				return nil
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			p, err := a.loadPolicy(name)
			if err != nil {
				return err
			}
			if !p.Learn {
				return fmt.Errorf("%s isn't learning (start with: limabean network learn %s)", name, name)
			}
			if !inst.Running() {
				return fmt.Errorf("%s is stopped, so its recorded lookups are gone; start it and run the task again", name)
			}
			counts, err := a.lookups(ctx, inst)
			if err != nil {
				return err
			}
			add := suggestAllow(p.Allow, counts)
			if len(add) == 0 && len(p.Allow) == 0 {
				return errors.New("nothing was looked up yet, so there's nothing to allow; use the VM first")
			}
			if err := a.changePolicy(ctx, name, func(p *netPolicy) error {
				p.Learn = false
				p.Allow = append(p.Allow, add...)
				return nil
			}); err != nil {
				return err
			}
			if len(add) > 0 {
				a.UI.Hint("Allowed: " + strings.Join(add, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&done, "done", false, "stop learning, and allow the domains that were looked up")
	return cmd
}
