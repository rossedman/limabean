package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newImagesRmCmd(a *App) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "rm <image>... | --all",
		Aliases: []string{"delete"},
		Short:   "Delete images",
		Long: `Delete images to free their disk space. VMs copied from them aren't
affected; the next VM of that kind is made the slow way, and builds a new
image in the background.`,
		Example: `  limabean images rm limabean-img-3fa2c1d0-t2k9x1
  limabean images rm --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if all == (len(args) > 0) {
				return errors.New("name images to delete, or pass --all")
			}
			names := args
			if all {
				list, err := a.allImageInstances(ctx)
				if err != nil {
					return err
				}
				names = list
			}
			for _, n := range names {
				if !isImage(n) {
					return fmt.Errorf("%q isn't an image (see: limabean images ls)", n)
				}
			}
			for _, n := range names {
				a.removeImage(ctx, n)
			}
			if len(names) == 0 {
				a.UI.Info("No images")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "delete every image")
	return cmd
}
