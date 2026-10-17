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

The VM's disk is erased. ~/code is kept when it's on its own disk, which
VMs created with the CODE_DISK setting on (the default) have; otherwise
repositories are lost and need cloning again with "limabean get". Its
secrets, Claude Desktop entry and settings are kept and reinstalled.

Without a code disk, rebuild first checks for uncommitted, stashed or
unpushed work, like rm. In a terminal it asks you to confirm. --force skips
both.`,
		Example: `  limabean rebuild dev

  # Pick the VM from a list
  limabean rebuild

  # No checks, no questions
  limabean rebuild dev --force`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
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
			keepsCode := a.hasCodeDisk(ctx, name)
			if !force {
				question := fmt.Sprintf("Rebuild %s from the current %s template? %s is kept; the rest of its disk is erased.", name, env, a.Cfg.CodeDir)
				if !keepsCode {
					clean, err := a.checkUnsaved(ctx, inst)
					if err != nil {
						return err
					}
					if !clean {
						return errSilent
					}
					question = fmt.Sprintf("Rebuild %s from the current %s template? Its disk is erased.", name, env)
				}
				if a.UI.Prompting {
					yes, err := a.UI.Confirm(question, false)
					if err != nil {
						return err
					}
					if !yes {
						a.UI.Info("Kept %s", name)
						return nil
					}
				}
			}
			// delete --force kills a running VM; shut it down first so
			// everything written to the code disk reaches it.
			if keepsCode && inst.Running() {
				if err := a.limaStep(ctx, "Stopping "+name, func(ctx context.Context) ([]byte, error) {
					return a.Lima.Stop(ctx, name)
				}); err != nil {
					return err
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
			if keepsCode {
				a.UI.Box(a.UI.Code(name)+" was rebuilt", "", "Repositories in "+a.Cfg.CodeDir+" were kept.")
			} else {
				a.UI.Box(a.UI.Code(name)+" was rebuilt", "", "Repositories weren't kept. Clone them again with:", "limabean get "+name+" <repo>")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip the unsaved-work check and the confirmation")
	return cmd
}
