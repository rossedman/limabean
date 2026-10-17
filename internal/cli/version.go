package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print limabean's version",
		Long: `Print limabean's version, and the Go version and platform it was built for.

Release builds carry their tag; builds from "go install" report the module
version; local builds report "dev".`,
		Example: `  limabean version
  limabean --version`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "limabean %s (%s, %s/%s)\n", a.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
