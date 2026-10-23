package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newImagesPruneCmd builds `limabean images prune`, which deletes images
// that are no longer used.
func newImagesPruneCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Delete images that are no longer used",
		Long: `Delete images built from templates that have changed since (they're never
used again), images nobody has copied for 30 days, and image VMs limabean
has no record of. "limabean autostop run" does this too.`,
		Example: `  limabean images prune`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n, err := a.pruneImages(cmd.Context())
			if err == nil && n == 0 {
				a.UI.Info("Nothing to prune")
			}
			return err
		},
	}
}

// pruneImages deletes outdated, unused and unknown images, and says how
// many.
func (a *App) pruneImages(ctx context.Context) (int, error) {
	root, err := a.templatesRoot()
	if err != nil {
		return 0, err
	}
	current, err := templatesHash(root)
	if err != nil {
		return 0, err
	}
	list, err := a.images(ctx)
	if err != nil {
		return 0, err
	}
	known := map[string]bool{}
	n := 0
	for _, im := range list {
		known[im.Name] = true
		if im.Templates != current || time.Since(im.Used) > 30*24*time.Hour {
			a.removeImage(ctx, im.Name)
			n++
		}
	}
	all, err := a.allImageInstances(ctx)
	if err != nil {
		return n, err
	}
	for _, name := range all {
		if known[name] {
			continue
		}
		// No record yet: it may be being built.
		if key, _, _ := strings.Cut(strings.TrimPrefix(name, imagePrefix), "-"); len(key) == 8 {
			unlock, err := lockFile(a.imageLock(key, "build"), true, false)
			if err != nil {
				continue
			}
			unlock()
		}
		a.removeImage(ctx, name)
		n++
	}
	return n, nil
}

// allImageInstances returns the Lima instances named as images, whether or
// not limabean has a record of them.
func (a *App) allImageInstances(ctx context.Context) ([]string, error) {
	list, err := a.Lima.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, inst := range list {
		if isImage(inst.Name) {
			out = append(out, inst.Name)
		}
	}
	return out, nil
}
