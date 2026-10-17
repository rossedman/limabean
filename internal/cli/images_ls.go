package cli

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// imageEntry is one image in images ls --json.
type imageEntry struct {
	Name   string    `json:"name"`
	Env    string    `json:"env"`
	Packs  []string  `json:"packs"`
	Built  time.Time `json:"built"`
	Used   time.Time `json:"used"`
	Status string    `json:"status"` // current, stale or outdated
}

func newImagesLsCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List images",
		Long: `List the images new VMs are copied from: the environment and packs each is
for, and how old it is. A stale image is over a week old and is replaced
the next time it's used; an outdated one was built from templates that
have changed since, and is no longer used ("limabean images prune"
deletes it).`,
		Example: `  limabean images ls`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			list, err := a.images(ctx)
			if err != nil {
				return err
			}
			current := ""
			if root, err := a.templatesRoot(); err == nil {
				current, _ = templatesHash(root)
			}
			entries := make([]imageEntry, 0, len(list))
			for _, im := range list {
				status := "current"
				switch {
				case im.Templates != current:
					status = "outdated"
				case im.stale():
					status = "stale"
				}
				entries = append(entries, imageEntry{Name: im.Name, Env: im.Env, Packs: im.Packs, Built: im.Built, Used: im.Used, Status: status})
			}
			if asJSON {
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			if len(entries) == 0 {
				a.UI.Info("No images yet. The first VM you create builds one in the background.")
				return nil
			}
			rows := make([][]string, len(entries))
			for i, e := range entries {
				rows[i] = []string{e.Name, e.Env, strings.Join(e.Packs, ","), roughDuration(time.Since(e.Built)) + " ago", e.Status}
			}
			a.UI.Table([]string{"IMAGE", "ENV", "PACKS", "BUILT", "STATUS"}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	return cmd
}
