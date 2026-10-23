package cli

import (
	"errors"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/secrets"
)

// newSecretsEditCmd builds `limabean secrets edit`, which opens a secrets
// file in the user's editor.
func newSecretsEditCmd(a *App) *cobra.Command {
	var vm string
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a secrets file in your editor",
		Long: `Open the shared secrets file, or with --vm that VM's own file, in $VISUAL or
$EDITOR (vi if neither is set). The file is created with a short header if
it doesn't exist yet.

Afterwards the file is checked, and, in a terminal, you're offered to
install the changes in running VMs.`,
		Example: `  limabean secrets edit

  EDITOR="code --wait" limabean secrets edit --vm dev`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			file := secrets.SharedFile(a.Cfg.Dir())
			if vm != "" {
				if err := validVMName(vm); err != nil {
					return err
				}
				file = secrets.VMFile(a.Cfg.Dir(), vm)
			}
			if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
				// Creates the file with its header; "_" is never a real name.
				if err := secrets.Put(file, "_", ""); err != nil {
					return err
				}
			}
			editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
			ed := exec.Command("sh", "-c", editor+` "$1"`, "sh", file)
			ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := ed.Run(); err != nil {
				return err
			}
			_ = os.Chmod(file, 0o600)
			list, errs := secrets.LoadFile(file)
			a.reportParse(errs)
			a.UI.OK("%s: %d secret(s)", shortPath(file), len(list))
			sync, err := a.UI.Confirm("Install the changes in running VMs now?", false)
			if err != nil || !sync {
				if err == nil {
					a.UI.Hint("Later: limabean secrets sync")
				}
				return err
			}
			return a.syncSecrets(cmd.Context(), "")
		},
	}
	cmd.Flags().StringVar(&vm, "vm", "", "edit this VM's own file")
	return cmd
}
