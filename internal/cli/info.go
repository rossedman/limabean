package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// noInfo is what the VM prints when it has no limabean-info.
const noInfo = "limabean-info: none"

// newInfoCmd builds `limabean info`, which shows what a VM knows about
// itself.
func newInfoCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "info [name]",
		Short: "Show what a VM knows about itself",
		Long: `Show what limabean-info prints inside a VM: its packs and tools, its
network policy, the names of its secrets (never their values) and, for a
project's VM, the repository, the clone, the ports and the logs.

The VM must be running. Inside a project's checkout, it uses the
project's VM when you don't name one; otherwise leave out the name to pick
one from a list. VMs made before limabean-info need a rebuild to have it.`,
		Example: `  limabean info dev

  # As JSON, for scripts
  limabean info dev --json | jq .network`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.projectVMArg(ctx, args, "show")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.Running() {
				return fmt.Errorf("%s is stopped; start it first (limabean start %s)", name, name)
			}
			run := "limabean-info"
			if asJSON {
				run += " --json"
			}
			out, err := a.Lima.Run(ctx, inst, nil, `if command -v limabean-info >/dev/null; then `+run+`; else echo "`+noInfo+`"; fi`)
			if err != nil {
				return fmt.Errorf("running limabean-info in %s: %w %s", name, err, strings.TrimSpace(string(out)))
			}
			if strings.TrimSpace(string(out)) == noInfo {
				return notFoundErr("%s was made before limabean-info; rebuild it to get it (limabean rebuild %s)", name, name)
			}
			if asJSON {
				// The VM's output is data: pass it on only if it's JSON.
				var b bytes.Buffer
				if err := json.Indent(&b, bytes.TrimSpace(out), "", "  "); err != nil {
					return fmt.Errorf("limabean-info in %s didn't print JSON: %w", name, err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), b.String())
				return nil
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
