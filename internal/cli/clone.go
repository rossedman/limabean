package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
)

// newCloneCmd builds `limabean clone`, which copies a VM.
func newCloneCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <name> <new-name>",
		Short: "Copy a VM, in seconds",
		Long: `Make a new VM that starts as an exact copy of another: its system, its
packs, its ~/code with every clone and uncommitted change, its secrets and
its network policy. The disks are APFS clones, so it takes seconds and only
uses space for what changes afterwards.

The VM being copied is shut down for those seconds and started again if it
was running. Its snapshots aren't copied.

Use it to try something risky on a copy, or to give several agents the same
starting point and keep the one that worked.`,
		Example: `  limabean clone dev dev-experiment

  # Three identical sandboxes
  for i in 1 2 3; do limabean clone agent agent-$i; done`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.cloneVM(cmd.Context(), args[0], args[1])
		},
	}
	return cmd
}

// cloneVM copies the VM src to a new VM dst, and starts dst.
func (a *App) cloneVM(ctx context.Context, src, dst string) error {
	if err := validVMName(dst); err != nil {
		return usageErr("VM name %q: %w", dst, err)
	}
	inst, err := a.vm(ctx, src)
	if err != nil {
		return err
	}
	if _, exists, err := a.Lima.Get(ctx, dst); err != nil {
		return err
	} else if exists {
		return conflictErr("a VM named %q already exists", dst)
	}
	if a.hasCodeDisk(ctx, dst) {
		return fmt.Errorf("a code disk for %q is left over from an earlier VM; delete it first: limactl disk delete %s", dst, codeDiskName(dst))
	}
	p, err := a.loadPolicy(src)
	if err != nil {
		return err
	}
	// The copy mounts its own policy and its own code disk. The policy mount's
	// location is {{.Param.networkDir}}, so setting the param is enough.
	sets := []string{fmt.Sprintf(`.param.networkDir = %q`, a.networkDir(dst))}
	code := a.hasCodeDisk(ctx, src)
	if code {
		sets = append(sets, fmt.Sprintf(`.param.codeDisk = %q | (.additionalDisks[] | select(.name == %q) | .name) = %q`, codeDiskName(dst), codeDiskName(src), codeDiskName(dst)))
	}
	err = a.whileStopped(ctx, inst, func() error {
		if code {
			if err := a.cloneCodeDisk(ctx, inst, dst); err != nil {
				return err
			}
		}
		if err := a.savePolicy(dst, p); err != nil {
			return err
		}
		if err := a.limaStep(ctx, "Copying "+src+" to "+dst, func(ctx context.Context) ([]byte, error) {
			return a.Lima.Clone(ctx, false, src, dst, sets...)
		}); err != nil {
			a.discardClone(ctx, dst, code)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	clone, err := a.vm(ctx, dst)
	if err != nil {
		return err
	}
	_ = os.RemoveAll(snapshotsDir(clone))
	if err := a.copyVMState(src, dst); err != nil {
		return err
	}
	a.UI.OK("Copied %s to %s", a.UI.Code(src), a.UI.Code(dst))
	if clone, err = a.ensureRunning(ctx, clone); err != nil {
		return err
	}
	if !p.open() || p.Locked {
		if err := a.applyPolicy(ctx, clone); err != nil {
			return err
		}
		a.UI.OK("Network: %s", p.summary())
	}
	a.UI.Box(a.UI.Code(dst)+" is ready", "", "ssh lima-"+dst+"     or: limabean ssh "+dst)
	return nil
}

// cloneCodeDisk gives dst a code disk that's an APFS clone of src's: a new
// Lima disk whose data file is then replaced.
func (a *App) cloneCodeDisk(ctx context.Context, src lima.Instance, dst string) error {
	files, err := a.vmDiskFiles(ctx, src)
	if err != nil {
		return err
	}
	fi, err := os.Stat(files["code"])
	if err != nil {
		return err
	}
	if out, err := a.Lima.DiskCreate(ctx, codeDiskName(dst), fmt.Sprint(fi.Size())); err != nil {
		return fmt.Errorf("creating %s's code disk: %w %s", dst, err, out)
	}
	target := filepath.Join(filepath.Dir(src.Dir), "_disks", codeDiskName(dst), "datadisk")
	tmp := target + ".limabean-clone"
	if err := a.cloneFile(ctx, files["code"], tmp); err != nil {
		_, _ = a.Lima.DiskDelete(ctx, codeDiskName(dst))
		return err
	}
	return os.Rename(tmp, target)
}

// discardClone removes what a failed clone left behind: limactl clone
// copies the VM before it checks the copy's settings, so the copy may exist.
func (a *App) discardClone(ctx context.Context, dst string, code bool) {
	if _, ok, _ := a.Lima.Get(ctx, dst); ok {
		_, _ = a.Lima.Delete(ctx, dst)
	}
	_ = os.RemoveAll(a.networkDir(dst))
	if code {
		_, _ = a.Lima.DiskDelete(ctx, codeDiskName(dst))
	}
}

// copyVMState gives dst src's saved environment, packs, project tools and
// own secrets.
func (a *App) copyVMState(src, dst string) error {
	if err := a.saveEnv(dst, a.envOf(src)); err != nil {
		return err
	}
	if packs, ok := a.packsOf(src); ok {
		if err := a.savePacks(dst, packs); err != nil {
			return err
		}
	}
	if err := a.saveTools(dst, a.toolsOf(src)); err != nil {
		return err
	}
	list, _ := secrets.LoadFile(secrets.VMFile(a.Cfg.Dir(), src))
	for _, s := range list {
		if err := secrets.Put(secrets.VMFile(a.Cfg.Dir(), dst), s.Name, s.Ref); err != nil {
			return err
		}
	}
	return nil
}
