package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newSnapshotLsCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "ls [name]",
		Aliases: []string{"list"},
		Short:   "List a VM's snapshots",
		Long: `List a VM's snapshots, oldest first, with when each was saved and whether
it includes ~/code (VMs without a code disk only have their system saved).`,
		Example: `  limabean snapshot ls dev

  # For scripts
  limabean snapshot ls dev --json | jq -r '.[-1].tag'`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.vmArg(ctx, args, "list snapshots of")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			list, err := listSnapshots(inst)
			if err != nil {
				return err
			}
			if asJSON {
				if list == nil {
					list = []snapshotMeta{}
				}
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(list)
			}
			if len(list) == 0 {
				a.UI.Info("%s has no snapshots. Save one with: limabean snapshot save %s", name, name)
				return nil
			}
			rows := make([][]string, 0, len(list))
			for _, m := range list {
				saved := "system"
				if m.Code {
					saved = "system, " + a.Cfg.CodeDir
				}
				rows = append(rows, []string{m.Tag, m.Created.Local().Format("2006-01-02 15:04"), saved})
			}
			a.UI.Table([]string{"SNAPSHOT", "SAVED", "INCLUDES"}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	return cmd
}
