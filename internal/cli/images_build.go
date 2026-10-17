package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/templates"
)

func newImagesBuildCmd(a *App) *cobra.Command {
	var env, packList string
	var with, without []string
	var ifNeeded bool
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image now",
		Long: `Build the image new VMs of an environment and set of packs are copied
from, replacing an older one. VMs get the default packs (the PACKS
setting) with --with and --without as for "limabean new", or exactly the
packs in --packs.

limabean builds images in the background by itself; build one by hand so
that even the first VM of a kind starts in seconds, or to pick up updates
before the week is up.`,
		Example: `  # The image "limabean new" uses
  limabean images build

  # The one "limabean new api --with rust" uses
  limabean images build --with rust`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			root, err := a.templatesRoot()
			if err != nil {
				return err
			}
			base := templates.SplitList(a.Cfg.Packs)
			if cmd.Flags().Changed("packs") {
				if len(with) > 0 || len(without) > 0 {
					return errors.New("--packs names every pack; leave out --with and --without")
				}
				base = templates.SplitList(packList)
			}
			packs, err := a.resolvePacks(base, with, without)
			if err != nil {
				return err
			}
			if _, err := templates.Path(root, env); err != nil {
				return err
			}
			hash, err := templatesHash(root)
			if err != nil {
				return err
			}
			key := imageKey(hash, env, packs)
			// One build per image at a time; a second waits, or with
			// --if-needed leaves it to the first.
			unlock, err := lockFile(a.imageLock(key, "build"), true, !ifNeeded)
			if errors.Is(err, errLocked) {
				a.UI.Info("That image is already being built")
				return nil
			} else if err != nil {
				return err
			}
			defer unlock()
			if im, ok := a.imageFor(ctx, key); ok && ifNeeded && !im.stale() {
				a.UI.Info("Image %s is up to date", im.Name)
				return nil
			}
			im, err := a.buildImage(ctx, root, env, packs)
			if err != nil {
				return fmt.Errorf("building the image: %w", err)
			}
			a.UI.Hint("New " + env + " VMs with these packs are copied from " + im.Name)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&env, "env", "scratch", "environment template")
	f.StringSliceVar(&with, "with", nil, "add these packs to the defaults")
	f.StringSliceVar(&without, "without", nil, "leave these packs out")
	f.StringVar(&packList, "packs", "", "exactly these packs besides core, e.g. go,node")
	f.BoolVar(&ifNeeded, "if-needed", false, "do nothing if the image exists and is under a week old, or is being built")
	return cmd
}
