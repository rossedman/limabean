package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newSnapshotSaveCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "save [name] [snapshot]",
		Short: "Save a snapshot of a VM",
		Long: `Save a VM's disks as a snapshot: its system, and ~/code when that's on its
own disk. A running VM is shut down for the few seconds it takes and then
started again.

The snapshot is named after the date and time unless you name it. Saving
under a name that's taken replaces that snapshot.`,
		Example: `  limabean snapshot save dev before-refactor

  # Named after the time, e.g. 2026-10-17-1530
  limabean snapshot save dev`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "snapshot")
			if err != nil {
				return err
			}
			tag := time.Now().Format("2006-01-02-1504")
			if len(args) > 1 {
				tag = args[1]
			}
			if err := validSnapshotTag(tag); err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			files, err := a.vmDiskFiles(ctx, inst)
			if err != nil {
				return err
			}
			dir := filepath.Join(snapshotsDir(inst), tag)
			err = a.whileStopped(ctx, inst, func() error {
				// Write a new copy beside any old one, then swap it in. Tags
				// can't start with a dot, so this never names a snapshot.
				tmp := filepath.Join(snapshotsDir(inst), "."+tag+".saving")
				_ = os.RemoveAll(tmp)
				if err := os.MkdirAll(tmp, 0o755); err != nil {
					return err
				}
				if err := a.UI.Spin(ctx, "Saving "+name+" as "+tag, func(ctx contextT) error {
					for saved, src := range files {
						if err := a.cloneFile(ctx, src, filepath.Join(tmp, saved)); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					_ = os.RemoveAll(tmp)
					return err
				}
				meta, _ := json.Marshal(snapshotMeta{Tag: tag, Created: time.Now(), Code: files["code"] != ""})
				if err := os.WriteFile(filepath.Join(tmp, "snapshot.json"), meta, 0o644); err != nil {
					return err
				}
				_ = os.RemoveAll(dir)
				return os.Rename(tmp, dir)
			})
			if err != nil {
				return err
			}
			what := "its system"
			if files["code"] != "" {
				what = "its system and " + a.Cfg.CodeDir
			}
			a.UI.OK("Saved %s as %s (%s)", a.UI.Code(name), a.UI.Code(tag), what)
			a.UI.Hint(fmt.Sprintf("Roll back with: limabean snapshot restore %s %s", name, tag))
			return nil
		},
	}
}
