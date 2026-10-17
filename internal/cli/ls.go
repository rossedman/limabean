package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/secrets"
)

func newLsCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List your VMs",
		Long: `List the VMs built from limabean's templates: their status, the environment
they were built from, size, how many secrets they get, and the SSH host to
connect to (ssh lima-<name>).

Other Lima instances aren't shown. ls also tidies up after VMs deleted with
"limactl delete": their Claude Desktop entries and saved state are removed.`,
		Example: `  limabean ls

  # Plain columns, for scripts
  limabean ls --plain | awk 'NR > 1 { print $1 }'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			a.prune(ctx)
			list, err := a.vms(ctx)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				a.UI.Info("No VMs yet. Create one with: limabean new <name>")
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
					fmt.Sprintf("%d GiB", inst.MemoryGiB()), strconv.Itoa(len(refs)), "lima-" + inst.Name,
				})
			}
			a.UI.Table([]string{"NAME", "STATUS", "ENV", "CPUS", "MEMORY", "SECRETS", "SSH HOST"}, rows)
			return nil
		},
	}
}
