package cli

import (
	"context"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// newCpCmd builds `limabean cp`, which copies files between the Mac and a
// VM.
func newCpCmd(a *App) *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:   "cp <source>... <target>",
		Short: "Copy files between your Mac and a VM",
		Long: `Copy files between your Mac and a VM, or between VMs, with limactl copy.
Write a VM path as <vm>:<path>; a relative one is in the VM user's home
directory. A stopped VM is started first.

Copy directories with -r.`,
		Example: `  # A file into the VM's home directory
  limabean cp ./notes.md dev:

  # A directory out of the VM
  limabean cp -r dev:code/github.com/me/app/dist ./dist

  # Between two VMs
  limabean cp dev:report.html agent:`,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var names []string
			for _, arg := range args {
				if name, ok := vmOfPath(arg); ok && !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
			if len(names) == 0 {
				return usageErr("name a VM path on one side, e.g. limabean cp ./file dev:")
			}
			for _, name := range names {
				inst, err := a.vm(ctx, name)
				if err != nil {
					return err
				}
				if _, err := a.ensureRunning(ctx, inst); err != nil {
					return err
				}
			}
			if err := a.limaStep(ctx, "Copying", func(ctx context.Context) ([]byte, error) {
				return a.Lima.Copy(ctx, recursive, args...)
			}); err != nil {
				return err
			}
			a.UI.OK("Copied to %s", args[len(args)-1])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "copy directories")
	return cmd
}

// vmOfPath returns the VM a <vm>:<path> argument names. Local paths with a
// colon need a slash before it (./a:b), as with scp.
func vmOfPath(arg string) (string, bool) {
	name, _, ok := strings.Cut(arg, ":")
	if !ok || validVMName(name) != nil {
		return "", false
	}
	return name, true
}
