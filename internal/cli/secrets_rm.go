package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/secrets"
)

func newSecretsRmCmd(a *App) *cobra.Command {
	var vm string
	var noSync bool
	cmd := &cobra.Command{
		Use:     "rm NAME",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove a secret",
		Long: `Remove a secret from the shared file, or with --vm from that VM's own file,
then update running VMs so it's gone from them too.

If the value lived in a Keychain item limabean created (limabean-secret-…)
and nothing else refers to it, the item is deleted as well. Items you
created yourself, 1Password items and the like are never touched.`,
		Example: `  limabean secrets rm OPENAI_API_KEY

  # Only dev's own copy
  limabean secrets rm NPM_TOKEN --vm dev`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSecretNames(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			file := secrets.SharedFile(a.Cfg.Dir())
			if vm != "" {
				file = secrets.VMFile(a.Cfg.Dir(), vm)
			}
			list, _ := secrets.LoadFile(file)
			var ref string
			found := false
			for _, s := range list {
				if s.Name == name {
					ref, found = s.Ref, true
				}
			}
			if !found {
				return fmt.Errorf("%s isn't in %s (see: limabean secrets ls)", name, shortPath(file))
			}
			if err := secrets.Put(file, name, ""); err != nil {
				return err
			}
			a.UI.OK("Removed %s from %s", name, shortPath(file))
			if a.ownsKeychainRef(ref) && !a.referencedElsewhere(ref, "") {
				if err := a.Keychain.Delete(strings.TrimPrefix(ref, "keychain:")); err == nil {
					a.UI.OK("Deleted its Keychain item")
				}
			}
			if noSync {
				return nil
			}
			return a.syncSecrets(cmd.Context(), "")
		},
	}
	cmd.Flags().StringVar(&vm, "vm", "", "remove it from this VM's own file")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "don't update running VMs yet")
	return cmd
}

// completeSecretNames tab-completes names from the shared file.
func completeSecretNames(a *App) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 || a.setup() != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		list, _ := secrets.Load(a.Cfg.Dir(), "")
		names := make([]string, len(list))
		for i, s := range list {
			names[i] = s.Name
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
