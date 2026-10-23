package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
)

// newSnapshotCmd builds `limabean snapshot`, the group of commands that save
// a VM's state and roll back to it.
func newSnapshotCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Save a VM's state and roll back to it later",
		Long: `Save a VM's disks as a snapshot, and restore one to put the VM back exactly
as it was: the system, and ~/code when it's on its own disk. Take one before
letting Claude loose on something risky, and roll back if it goes wrong.

Snapshots are APFS clones of the VM's disks, so they take a few seconds and
only use space for what changes afterwards. The VM is shut down while one
is saved or restored, and started again if it was running.

Snapshots live with the VM: rm and rebuild delete them.`,
		Example: `  limabean snapshot save dev before-refactor
  limabean snapshot ls dev
  limabean snapshot restore dev before-refactor
  limabean snapshot rm dev before-refactor`,
	}
	cmd.AddCommand(newSnapshotSaveCmd(a), newSnapshotLsCmd(a), newSnapshotRestoreCmd(a), newSnapshotRmCmd(a))
	return cmd
}

// snapshotMeta is saved with each snapshot.
type snapshotMeta struct {
	Tag     string    `json:"tag"`
	Created time.Time `json:"created"`
	Code    bool      `json:"code"` // includes the code disk
}

var snapshotTagRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// validSnapshotTag refuses a snapshot name that isn't plain letters, digits,
// dots, dashes and underscores, so it can't act as a flag or a path.
func validSnapshotTag(tag string) error {
	if !snapshotTagRE.MatchString(tag) {
		return fmt.Errorf("snapshot name %q: use letters, digits, dots, dashes and underscores", tag)
	}
	return nil
}

// snapshotsDir holds a VM's snapshots, one directory each.
func snapshotsDir(inst lima.Instance) string { return filepath.Join(inst.Dir, "limabean-snapshots") }

// vmDiskFiles maps the name each of a VM's disk files is saved under to
// where it is now: the system disk (Lima's "disk", or "diffdisk" in older
// instances, possibly a symlink), the EFI variable store, and the code
// disk if the VM has one.
func (a *App) vmDiskFiles(ctx context.Context, inst lima.Instance) (map[string]string, error) {
	files := map[string]string{}
	for _, name := range []string{"disk", "diffdisk"} {
		if p, err := filepath.EvalSymlinks(filepath.Join(inst.Dir, name)); err == nil {
			files["disk"] = p
			break
		}
	}
	if files["disk"] == "" {
		return nil, fmt.Errorf("can't find %s's disk in %s", inst.Name, inst.Dir)
	}
	if p := filepath.Join(inst.Dir, "vz-efi"); fileExists(p) {
		files["vz-efi"] = p
	}
	if a.hasCodeDisk(ctx, inst.Name) {
		// Lima keeps disks in $LIMA_HOME/_disks, next to the VMs.
		files["code"] = filepath.Join(filepath.Dir(inst.Dir), "_disks", codeDiskName(inst.Name), "datadisk")
	}
	return files, nil
}

// listSnapshots returns a VM's snapshots, oldest first.
func listSnapshots(inst lima.Instance) ([]snapshotMeta, error) {
	entries, err := os.ReadDir(snapshotsDir(inst))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var list []snapshotMeta
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(snapshotsDir(inst), e.Name(), "snapshot.json"))
		if err != nil {
			continue // half-written, or not ours
		}
		var m snapshotMeta
		if json.Unmarshal(b, &m) == nil && m.Tag == e.Name() {
			list = append(list, m)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.Before(list[j].Created) })
	return list, nil
}

// whileStopped runs fn with the VM shut down, starting it again afterwards
// if it was running.
func (a *App) whileStopped(ctx context.Context, inst lima.Instance, fn func() error) error {
	if inst.Running() {
		if err := a.limaStep(ctx, "Stopping "+inst.Name, func(ctx context.Context) ([]byte, error) {
			return a.Lima.Stop(ctx, inst.Name)
		}); err != nil {
			return err
		}
		defer func() {
			if _, err := a.ensureRunning(ctx, lima.Instance{Name: inst.Name}); err != nil {
				a.UI.Warn("Couldn't start %s again: %v", inst.Name, err)
				a.UI.Hint("limabean up " + inst.Name)
			} else {
				a.UI.OK("Started %s again", a.UI.Code(inst.Name))
			}
		}()
	}
	return fn()
}

// cloneFile makes dst an APFS clone of src (cp -c): instant, and sharing
// blocks until either changes.
func cloneFile(ctx context.Context, src, dst string) error {
	if out, err := exec.CommandContext(ctx, "cp", "-c", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("cloning %s: %v: %s (snapshots need an APFS disk)", filepath.Base(src), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// fileExists reports whether p can be stat'd.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
