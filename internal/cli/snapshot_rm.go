package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newSnapshotRmCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name> <snapshot>",
		Short: "Delete a snapshot",
		Long: `Delete one of a VM's snapshots, freeing the space it kept. The VM itself is
left alone.`,
		Example: `  limabean snapshot rm dev before-refactor`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, tag := args[0], args[1]
			if err := validSnapshotTag(tag); err != nil {
				return err
			}
			inst, err := a.vm(cmd.Context(), name)
			if err != nil {
				return err
			}
			dir := filepath.Join(snapshotsDir(inst), tag)
			if !fileExists(filepath.Join(dir, "snapshot.json")) {
				return fmt.Errorf("%s has no snapshot %q (see: limabean snapshot ls %s)", name, tag, name)
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			a.UI.OK("Deleted snapshot %s of %s", a.UI.Code(tag), name)
			return nil
		},
	}
}
