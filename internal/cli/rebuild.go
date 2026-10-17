package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newRebuildCmd(a *App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rebuild [name]",
		Short: "Recreate a VM from the current templates",
		Long: `Delete a VM and create it again, with the same name and environment, from
the templates as they are now. This is how changes to templates, tools,
shell config or limabean itself reach an existing VM.

The VM's disk is erased, so repositories in ~/code are lost; clone them
again with "limabean get". Its secrets, Claude Desktop entry and settings
are kept and reinstalled.

Like rm, rebuild first checks for uncommitted, stashed or unpushed work and,
in a terminal, asks you to confirm. --force skips both.`,
		Example: `  limabean rebuild dev

  # Pick the VM from a list
  limabean rebuild

  # No checks, no questions
  limabean rebuild dev --force`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "rebuild")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			env := a.envOf(name)
			if env == "" {
				env = "scratch"
			}
			if !force {
				clean, err := a.checkUnsaved(ctx, inst)
				if err != nil {
					return err
				}
				if !clean {
					return errSilent
				}
				if a.UI.Prompting {
					yes, err := a.UI.Confirm(fmt.Sprintf("Rebuild %s from the current %s template? Its disk is erased.", name, env), false)
					if err != nil {
						return err
					}
					if !yes {
						a.UI.Info("Kept %s", name)
						return nil
					}
				}
			}
			if err := a.limaStep(ctx, "Deleting "+name, func(ctx context.Context) ([]byte, error) {
				return a.Lima.Delete(ctx, name)
			}); err != nil {
				return err
			}
			if err := a.create(ctx, createOptions{Name: name, Env: env}); err != nil {
				return err
			}
			a.UI.Box(a.UI.Code(name)+" was rebuilt", "", "Repositories weren't kept. Clone them again with:", "limabean get "+name+" <repo>")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip the unsaved-work check and the confirmation")
	return cmd
}
