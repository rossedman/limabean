package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
)

func newSnapshotRestoreCmd(a *App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "restore [name] [snapshot]",
		Short: "Put a VM back the way a snapshot saved it",
		Long: `Restore a snapshot: the VM's system, and ~/code if the snapshot includes it,
go back to how they were when it was saved. Everything changed since then
is lost, pushed or not. A running VM is shut down and started again.

Leave out the snapshot to restore the latest one. In a terminal it asks
you to confirm; --force doesn't ask.`,
		Example: `  limabean snapshot restore dev before-refactor

  # The latest snapshot, without asking
  limabean snapshot restore dev --force`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "restore")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			list, err := listSnapshots(inst)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				return fmt.Errorf("%s has no snapshots (limabean snapshot save %s)", name, name)
			}
			snap := list[len(list)-1]
			if len(args) > 1 {
				i := slices.IndexFunc(list, func(m snapshotMeta) bool { return m.Tag == args[1] })
				if i < 0 {
					return fmt.Errorf("%s has no snapshot %q (see: limabean snapshot ls %s)", name, args[1], name)
				}
				snap = list[i]
			}
			files, err := a.vmDiskFiles(ctx, inst)
			if err != nil {
				return err
			}
			if snap.Code && files["code"] == "" {
				return errors.New("the snapshot includes ~/code, but the VM no longer has a code disk to restore it to")
			}
			if !force && a.UI.Prompting {
				yes, err := a.UI.Confirm(fmt.Sprintf("Restore %s to %s from %s? Everything changed since is lost.",
					name, snap.Tag, snap.Created.Local().Format("2006-01-02 15:04")), false)
				if err != nil {
					return err
				}
				if !yes {
					a.UI.Info("Kept %s as it is", name)
					return nil
				}
			}
			dir := filepath.Join(snapshotsDir(inst), snap.Tag)
			err = a.whileStopped(ctx, inst, func() error {
				return a.UI.Spin(ctx, "Restoring "+name+" to "+snap.Tag, func(ctx contextT) error {
					for saved, dst := range files {
						src := filepath.Join(dir, saved)
						if !fileExists(src) {
							continue // e.g. a code disk added after the snapshot
						}
						// Clone beside the file, then rename over it, so a
						// failure never leaves a half-written disk.
						tmp := dst + ".limabean-restore"
						_ = os.Remove(tmp)
						if err := a.cloneFile(ctx, src, tmp); err != nil {
							return err
						}
						if err := os.Rename(tmp, dst); err != nil {
							_ = os.Remove(tmp)
							return err
						}
					}
					return nil
				})
			})
			if err != nil {
				return err
			}
			a.UI.OK("Restored %s to %s", a.UI.Code(name), a.UI.Code(snap.Tag))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "don't ask for confirmation")
	return cmd
}
