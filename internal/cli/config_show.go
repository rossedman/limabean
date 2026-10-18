package cli

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/config"
)

// settingEntry is one setting in config --json.
type settingEntry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"` // flag, env, file or default
}

func newConfigCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show your settings and where each comes from",
		Long: `Show every setting, its value, and where the value comes from: a flag, a
LIMABEAN_* environment variable, the config file, or limabean's default.
Earlier ones win, in that order.

The config file is written by "limabean init"; edit it, or run init again,
to change a setting.`,
		Example: `  limabean config

  # One setting, for scripts
  limabean config --json | jq -r '.[] | select(.name == "CODE_DIR") | .value'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Root().PersistentFlags()
			entries := make([]settingEntry, 0, len(config.Keys))
			for _, key := range config.Keys {
				source := config.Source(a.V, key)
				if f := flags.Lookup(key); f != nil && f.Changed {
					source = "flag"
				}
				entries = append(entries, settingEntry{Name: strings.ToUpper(key), Value: a.V.GetString(key), Source: source})
			}
			if asJSON {
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{e.Name, `"` + e.Value + `"`, e.Source})
			}
			a.UI.Table([]string{"SETTING", "VALUE", "SOURCE"}, rows)
			if _, err := os.Stat(a.Cfg.Path); err != nil {
				a.UI.Hint("File: " + shortPath(a.Cfg.Path) + " (none yet; limabean init writes it)")
			} else {
				a.UI.Hint("File: " + shortPath(a.Cfg.Path))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	return cmd
}
