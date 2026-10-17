package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newRmCmd(a *App) *cobra.Command {
	var force, expired bool
	cmd := &cobra.Command{
		Use:     "rm [name]",
		Aliases: []string{"delete"},
		Short:   "Delete a VM",
		Long: `Delete a VM and everything limabean keeps for it: the disk its ~/code is
on, its Claude Desktop entry, its saved environment, its own secrets file
(secrets.<name>.env) and the Keychain items limabean created for those
secrets. Shared secrets are kept.

Before deleting, rm checks every git repository under ~/code in the VM. If
any has uncommitted changes, stashes or commits that aren't on a remote, it
stops and lists them. In a terminal it then asks you to confirm. --force
skips both.

A VM already deleted with "limactl delete" can still be named: rm cleans up
what's left of it.

--expired deletes every VM made with "new --ttl" whose time is up, without
checks; the autostop agent does this every 10 minutes.`,
		Example: `  limabean rm dev

  # Pick the VM from a list
  limabean rm

  # No checks, no questions
  limabean rm scratch --force

  # Every VM past its --ttl
  limabean rm --expired`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if expired {
				if len(args) > 0 {
					return errors.New("--expired deletes every expired VM; leave out the name")
				}
				return a.removeExpired(ctx)
			}
			name, err := a.vmArg(ctx, args, "delete")
			if err != nil {
				return err
			}
			inst, ok, err := a.Lima.Get(ctx, name)
			if err != nil {
				return err
			}
			if !ok {
				if a.envOf(name) == "" && !a.hasLeftovers(ctx, name) {
					return fmt.Errorf("no VM named %q (see: limabean ls)", name)
				}
				a.forget(ctx, name)
				a.UI.OK("No VM named %s; removed what was left of it", name)
				return nil
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
					yes, err := a.UI.Confirm(fmt.Sprintf("Delete %s? This can't be undone.", name), false)
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
			a.forget(ctx, name)
			a.UI.OK("Deleted %s", a.UI.Code(name))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip the unsaved-work check and the confirmation")
	cmd.Flags().BoolVar(&expired, "expired", false, "delete every VM whose --ttl is up")
	return cmd
}
