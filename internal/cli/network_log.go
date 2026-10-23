package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// lookupEntry is one name in network log --json.
type lookupEntry struct {
	Name    string `json:"name"`
	Lookups int    `json:"lookups"`
	Allowed bool   `json:"allowed"`
}

// newNetworkLogCmd builds `limabean network log`, which shows the names a VM
// looked up and which were blocked.
func newNetworkLogCmd(a *App) *cobra.Command {
	var asJSON, blocked bool
	cmd := &cobra.Command{
		Use:   "log <name>",
		Short: "Show the names a VM looked up, and which were blocked",
		Long: `Show every name a running VM, or a container in it, looked up since it
started, how often, and whether its allowlist let it through. When an
install or a clone fails in a sandbox, the domain it needed is here.

Lookups are recorded while the VM has an allowlist or is learning
(limabean network learn); an open VM's aren't.`,
		Example: `  limabean network log agent

  # Only what the allowlist stopped
  limabean network log agent --blocked`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.Running() {
				return fmt.Errorf("%s is stopped; lookups are recorded while it runs", name)
			}
			p, err := a.loadPolicy(name)
			if err != nil {
				return err
			}
			if p.open() {
				return fmt.Errorf("%s's network is open, so its lookups aren't recorded (record them with: limabean network learn %s)", name, name)
			}
			counts, err := a.lookups(ctx, inst)
			if err != nil {
				return err
			}
			entries := []lookupEntry{}
			allowed := append([]string{"ntp.ubuntu.com"}, p.Allow...)
			for n, c := range counts {
				ok := p.Learn || allows(allowed, n)
				if !blocked || !ok {
					entries = append(entries, lookupEntry{Name: n, Lookups: c, Allowed: ok})
				}
			}
			slices.SortFunc(entries, func(x, y lookupEntry) int { return strings.Compare(x.Name, y.Name) })
			if asJSON {
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			if len(entries) == 0 {
				a.UI.Info("No lookups to show")
				return nil
			}
			rows := make([][]string, len(entries))
			for i, e := range entries {
				status := "allowed"
				switch {
				case p.Learn:
					status = "seen"
				case !e.Allowed:
					status = "blocked"
				}
				rows[i] = []string{e.Name, strconv.Itoa(e.Lookups), status}
			}
			a.UI.Table([]string{"NAME", "LOOKUPS", "STATUS"}, rows)
			if add := suggestAllow(p.Allow, counts); len(add) > 0 {
				if p.Learn {
					a.UI.Hint("limabean network learn " + name + " --done   allows: " + strings.Join(add, ", "))
				} else {
					a.UI.Hint("To allow the blocked ones: limabean network allow " + name + " " + strings.Join(add, " "))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	cmd.Flags().BoolVar(&blocked, "blocked", false, "only show names the allowlist blocked")
	return cmd
}
