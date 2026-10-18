package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/project"
)

func newDownCmd(a *App) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop the VM a repository's .limabean.yaml describes",
		Long: `Stop the project's VM: the one named by .limabean.yaml (or .limabean.yml)
here or in a parent directory. Everything in it is kept, ~/code included,
and "limabean up" starts it again, applying any changes to the file.

down is "limabean stop <name>" for whichever project you're in. To delete
the VM, use limabean rm.`,
		Example: `  cd ~/src/myapp && limabean down

  # Bring it back, with whatever changed in the file
  limabean up`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			path := file
			if path == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				if path, err = project.Find(wd); err != nil {
					return err
				}
			}
			f, err := project.Load(path)
			if err != nil {
				return err
			}
			inst, ok, err := a.Lima.Get(ctx, f.Name)
			switch {
			case err != nil:
				return err
			case !ok:
				a.UI.Info("%s has no VM yet; limabean up creates it", f.Name)
				return nil
			case !inst.Running():
				a.UI.Info("%s is already stopped", a.UI.Code(f.Name))
				return nil
			}
			return a.stopVM(ctx, f.Name, "Stopping "+f.Name)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "the project file (default: .limabean.yaml here or in a parent directory)")
	return cmd
}
