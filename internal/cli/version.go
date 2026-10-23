package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print limabean's version",
		Long: `Print limabean's version, and the Go version and platform it was built for.

Release builds carry their tag; builds from "go install" report the module
version; local builds report "dev". With --json it prints them as a JSON object.`,
		Example: `  limabean version
  limabean --version
  limabean version --json | jq -r .version`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"version": a.Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
				})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "limabean %s (%s, %s/%s)\n", a.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON object, for scripts")
	return cmd
}
