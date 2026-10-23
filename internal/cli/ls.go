package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
)

// lsEntry is one VM in ls --json.
type lsEntry struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Env       string   `json:"env"`
	CPUs      int      `json:"cpus"`
	MemoryGiB int64    `json:"memoryGiB"`
	Secrets   int      `json:"secrets"`
	SSHHost   string   `json:"sshHost"`
	Network   string   `json:"network"`           // e.g. "open", "3 domain(s), locked"
	Packs     []string `json:"packs"`             // besides core; null if it was made before packs
	Expires   string   `json:"expires,omitempty"` // when new --ttl deletes it, RFC 3339
}

// newLsCmd builds `limabean ls`, which lists the VMs.
func newLsCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List your VMs",
		Long: `List the VMs built from limabean's templates: their status, the environment
they were built from, size, how many secrets they get, and the SSH host to
connect to (ssh lima-<name>).

Other Lima instances aren't shown. ls also tidies up after VMs deleted with
"limactl delete": their Claude Desktop entries and saved state are removed.

With --json it prints a JSON array instead, one object per VM, and notes
about tidying up go to stderr. An environment that isn't known is "".`,
		Example: `  limabean ls

  # Plain columns, for scripts
  limabean ls --plain | awk 'NR > 1 { print $1 }'

  # JSON, for scripts: the names of running VMs
  limabean ls --json | jq -r '.[] | select(.status == "Running") | .name'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			a.prune(ctx)
			list, err := a.vms(ctx)
			if err != nil {
				return err
			}
			if asJSON {
				entries := make([]lsEntry, 0, len(list))
				for _, inst := range list {
					refs, _ := secrets.Load(a.Cfg.Dir(), inst.Name)
					entries = append(entries, lsEntry{
						Name: inst.Name, Status: inst.Status, Env: a.envOf(inst.Name), CPUs: inst.CPUs,
						MemoryGiB: inst.MemoryGiB(), Secrets: len(refs), SSHHost: "lima-" + inst.Name,
						Network: a.networkSummary(inst),
						Packs:   a.packsOrNil(inst.Name),
					})
					if t, ok := a.expiryOf(inst.Name); ok {
						entries[len(entries)-1].Expires = t.Format(time.RFC3339)
					}
				}
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			if len(list) == 0 {
				a.UI.Info("No VMs yet. Create one with: limabean up <name>")
				return nil
			}
			rows := make([][]string, 0, len(list))
			for _, inst := range list {
				env := a.envOf(inst.Name)
				if env == "" {
					env = "-"
				}
				refs, _ := secrets.Load(a.Cfg.Dir(), inst.Name)
				rows = append(rows, []string{
					inst.Name, inst.Status, env, strconv.Itoa(inst.CPUs),
					fmt.Sprintf("%d GiB", inst.MemoryGiB()), strconv.Itoa(len(refs)), a.networkSummary(inst), "lima-" + inst.Name,
				})
			}
			a.UI.Table([]string{"NAME", "STATUS", "ENV", "CPUS", "MEMORY", "SECRETS", "NETWORK", "SSH HOST"}, rows)
			// Not in plain output, where scripts read the rows.
			for _, inst := range list {
				if t, ok := a.expiryOf(inst.Name); ok && a.UI.Prompting {
					a.UI.Hint(fmt.Sprintf("%s is deleted in %s (--ttl)", inst.Name, roughDuration(max(time.Until(t), 0))))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	return cmd
}

// networkSummary describes a VM's network policy for ls.
func (a *App) networkSummary(inst lima.Instance) string {
	p, err := a.loadPolicy(inst.Name)
	if err != nil || !inst.NetworkPolicy {
		return "open"
	}
	return p.summary()
}

// packsOrNil is a VM's packs for ls --json: [] for none, null if unknown.
func (a *App) packsOrNil(name string) []string {
	packs, known := a.packsOf(name)
	if !known {
		return nil
	}
	if packs == nil {
		packs = []string{}
	}
	return packs
}
