package cli

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/secrets"
)

func newSecretsLsCmd(a *App) *cobra.Command {
	var vm string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List secrets and where their values come from",
		Long: `List every secret with where its value comes from and which VMs get it.
Values are never shown, and plain values are only marked as such.

With --vm, list exactly what that VM gets: the shared secrets merged with
its own, its own winning when both set the same name.`,
		Example: `  limabean secrets ls

  # What dev gets
  limabean secrets ls --vm dev`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := a.Cfg.Dir()
			var rows [][]string
			if vm != "" {
				list, errs := secrets.Load(dir, vm)
				a.reportParse(errs)
				for _, s := range list {
					scope := "every VM"
					if s.Source != "secrets.env" {
						scope = vm + " only"
					}
					rows = append(rows, []string{s.Name, secrets.Kind(s.Ref), scope})
				}
			} else {
				files, _ := filepath.Glob(filepath.Join(dir, "secrets*.env"))
				slices.SortFunc(files, func(x, y string) int { // shared file first
					if filepath.Base(x) == "secrets.env" {
						return -1
					}
					if filepath.Base(y) == "secrets.env" {
						return 1
					}
					return strings.Compare(x, y)
				})
				for _, f := range files {
					list, errs := secrets.LoadFile(f)
					a.reportParse(errs)
					scope := "every VM"
					if base := filepath.Base(f); base != "secrets.env" {
						scope = strings.TrimSuffix(strings.TrimPrefix(base, "secrets."), ".env") + " only"
					}
					for _, s := range list {
						rows = append(rows, []string{s.Name, secrets.Kind(s.Ref), scope})
					}
				}
			}
			if len(rows) == 0 {
				a.UI.Info("No secrets yet. Add one with: limabean secrets set NAME")
				return nil
			}
			a.UI.Table([]string{"NAME", "SOURCE", "VMS"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&vm, "vm", "", "show what this VM gets")
	return cmd
}

// reportParse warns about lines in secrets files that don't parse.
func (a *App) reportParse(errs []error) {
	for _, e := range errs {
		a.UI.Warn("%v", e)
	}
}
