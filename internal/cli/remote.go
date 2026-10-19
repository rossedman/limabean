package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/project"
)

// remoteNameRE is what --name may be: a plain git remote name, never a flag.
var remoteNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// clonePathRE is a clone path limabean will put in a remote's URL. The path
// comes from the VM (ghq list), so it's data: an absolute path of plain
// characters, nothing git or ssh could read as an option or a second host.
var clonePathRE = regexp.MustCompile(`^/[A-Za-z0-9._@+/-]+$`)

func newRemoteCmd(a *App) *cobra.Command {
	var remote string
	var remove bool
	cmd := &cobra.Command{
		Use:   "remote [name]",
		Short: "Add a VM's clone of this repository as a git remote on your Mac",
		Long: `Add a VM's clone of the repository you're in as a git remote of your
checkout on the Mac, so work done in the VM (by you or by Claude) can be
fetched and reviewed on the Mac without pushing it to GitHub first.

Run it in a checkout. With a .limabean.yaml, it uses the project's VM;
otherwise, or to use another VM, name one. The VM is started if it's
stopped, and must have cloned the repository (limabean up does).

The remote, called limabean unless you pass --name, points at
lima-<vm>:<clone> over SSH, the way limabean ssh and your editor reach the
VM, so it works whatever the VM's network allows. It fetches branches
only, not tags, so nothing in the VM can add tags to your checkout.
Fetching copies git objects into the Mac's checkout; it runs nothing from
the VM.

Run it again after a rebuild to point the remote at the new clone.`,
		Example: `  # In a project's checkout: add the VM's clone, then fetch its branches
  limabean remote
  git fetch limabean
  git diff main...limabean/my-branch

  # Hand a branch to the VM
  git push limabean my-branch

  # Another VM, under another name
  limabean remote scratch --name scratch

  # Remove it
  limabean remote --rm`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// It runs git on the Mac, which nothing driven over MCP may do.
			if a.mcpMode {
				return errors.New("remote runs git on the Mac, so it isn't available over MCP")
			}
			if !remoteNameRE.MatchString(remote) {
				return usageErr("--name %q isn't a git remote name: use letters, digits, '.', '_' and '-'", remote)
			}
			f, dir := &project.File{}, "."
			if path, err := project.Find("."); err == nil {
				if f, err = project.Load(path); err != nil {
					return err
				}
				dir = filepath.Dir(path)
			}
			top, err := a.git(ctx, dir, "rev-parse", "--show-toplevel")
			if err != nil {
				return notFoundErr("not in a git checkout: run limabean remote in the checkout whose VM clone you want")
			}
			dir = strings.TrimSpace(string(top))
			// The URL as configured: get-url would apply insteadOf rewrites.
			have, getErr := a.git(ctx, dir, "config", "--get", "remote."+remote+".url")
			current := strings.TrimSpace(string(have))
			if remove {
				return a.removeRemote(ctx, dir, remote, current, getErr == nil)
			}

			name, err := a.projectVMArg(ctx, args, "add as a remote")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if inst, err = a.ensureRunning(ctx, inst); err != nil {
				return err
			}
			clone, err := a.projectClone(ctx, inst, f, dir)
			if err != nil {
				return err
			}
			if !clonePathRE.MatchString(clone) || strings.Contains(clone, "..") {
				return fmt.Errorf("%s's clone is at %q, which limabean won't put in a git remote", inst.Name, clone)
			}
			url := "lima-" + inst.Name + ":" + clone
			switch {
			case getErr == nil && current == url:
				a.UI.OK("%s is already a remote: %s", a.UI.Code(remote), url)
			case getErr == nil && !strings.HasPrefix(current, "lima-"):
				return conflictErr("this checkout already has a remote named %s (%s); pick another name with --name", remote, current)
			case getErr == nil:
				if _, err := a.git(ctx, dir, "remote", "set-url", "--", remote, url); err != nil {
					return fmt.Errorf("changing the remote %s: %w", remote, err)
				}
				a.UI.OK("Pointed %s at %s", a.UI.Code(remote), url)
			default:
				if _, err := a.git(ctx, dir, "remote", "add", "--no-tags", "--", remote, url); err != nil {
					return fmt.Errorf("adding the remote %s: %w", remote, err)
				}
				a.UI.OK("Added %s: %s", a.UI.Code(remote), url)
			}
			if _, err := a.git(ctx, dir, "config", "remote."+remote+".tagOpt", "--no-tags"); err != nil {
				return fmt.Errorf("setting the remote %s to fetch no tags: %w", remote, err)
			}
			a.UI.Hint("git fetch "+remote+"                       # the VM's branches, as "+remote+"/<branch>",
				"git diff main..."+remote+"/<branch>       # what changed there",
				"git push "+remote+" <branch>              # hand a branch to the VM (not the one checked out there)")
			return nil
		},
	}
	cmd.Flags().StringVar(&remote, "name", "limabean", "the remote's name")
	cmd.Flags().BoolVar(&remove, "rm", false, "remove the remote instead")
	return cmd
}

// removeRemote removes a remote limabean added; one pointing anywhere else
// is left alone.
func (a *App) removeRemote(ctx context.Context, dir, remote, current string, exists bool) error {
	if !exists {
		a.UI.Info("This checkout has no remote named %s", remote)
		return nil
	}
	if !strings.HasPrefix(current, "lima-") {
		return conflictErr("the remote %s points at %s, not a VM, so limabean leaves it alone", remote, current)
	}
	if _, err := a.git(ctx, dir, "remote", "remove", "--", remote); err != nil {
		return fmt.Errorf("removing the remote %s: %w", remote, err)
	}
	a.UI.OK("Removed %s", a.UI.Code(remote))
	return nil
}

// runGit runs git on the Mac in a checkout, returning its standard output.
func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
}
