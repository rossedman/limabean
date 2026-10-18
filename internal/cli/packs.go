package cli

import (
	"encoding/json"
	"slices"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/templates"
)

// packEntry is one pack in packs --json.
type packEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Default     bool     `json:"default"` // new VMs get it (core always does)
	Tools       []string `json:"tools"`
	Apt         []string `json:"apt"`
}

func newPacksCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "packs",
		Short: "List the packs of tools a VM can get",
		Long: `List the packs of tools VMs can get and what each installs. Every VM has
core, the shell tools limabean itself relies on, and Docker and Python;
everything else is a pack you choose:

  limabean new api --with go,db          # or pick them in new's form
  limabean rebuild dev --with ruby       # add one to an existing VM
  packs: [go, db]                        # in a project's .limabean.yaml

A VM keeps its packs through rebuilds. The PACKS setting adds packs to
every VM made with new or run (it's empty unless you set it; project files
ignore it). Versions are pinned in templates/packs/*.toml, and
"limabean bump" updates them.`,
		Example: `  limabean packs

  # Give every new VM Go and Node
  echo 'PACKS="go,node"' >> ~/.config/limabean/config`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := a.templatesRoot()
			if err != nil {
				return err
			}
			all, err := templates.Packs(root)
			if err != nil {
				return err
			}
			defaults := templates.SplitList(a.Cfg.Packs)
			entries := make([]packEntry, 0, len(all))
			for _, p := range all {
				entries = append(entries, packEntry{
					Name: p.Name, Description: p.Description, Tools: p.Tools, Apt: p.Apt,
					Default: p.Name == templates.CorePack || slices.Contains(defaults, p.Name),
				})
			}
			if asJSON {
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			rows := make([][]string, 0, len(all))
			for i, p := range all {
				in := "-"
				switch {
				case p.Name == templates.CorePack:
					in = "always"
				case entries[i].Default:
					in = "yes"
				}
				rows = append(rows, []string{p.Name, in, p.Contents()})
			}
			a.UI.Table([]string{"PACK", "DEFAULT", "INSTALLS"}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	return cmd
}
