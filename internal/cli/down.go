package cli

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
)

func newDownCmd(a *App) *cobra.Command {
	var file string
	var idle time.Duration
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "down [name]",
		Short: "Stop a VM, or the project's",
		Long: `Stop a VM. Everything in it is kept, ~/code included, and "limabean up"
starts it again (in a project, applying any changes to its file). To
delete a VM, use limabean rm.

Without a name, down stops the project's VM: the one .limabean.yaml (or
.limabean.yml) here or in a parent directory describes. Outside a project
it asks which VM in a terminal.

Its Claude Desktop connection is removed while it's stopped (Claude Desktop
shows an error for a VM it can't reach) and added back when it starts.

With --idle, only VMs nobody has used for that long are stopped, and
without a name every running VM is checked. A VM is in use while:
  - a terminal in it (a shell, tmux, limabean ssh) had input or output
  - Claude Desktop or an editor is connected to it
  - its 5-minute load average is 1 or more, e.g. a build or busy containers
  - it started less than that long ago
"limabean autostop on" runs this every few minutes.`,
		Example: `  # The project you're in
  cd ~/src/myapp && limabean down

  # A VM by name
  limabean down dev

  # Every VM unused for two hours, and a dry run of the same
  limabean down --idle 2h
  limabean down --idle 2h --dry-run`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dryRun && idle == 0 {
				return errors.New("--dry-run needs --idle")
			}
			if cmd.Flags().Changed("idle") {
				if idle < time.Minute {
					return errors.New("--idle needs a duration of a minute or more, e.g. 2h or 90m")
				}
				return a.stopIdle(ctx, args, idle, dryRun)
			}
			name, err := a.targetVM(ctx, args, file, "down", "stop")
			if err != nil || name == "" {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.Running() {
				a.claudeRemove(name)
				a.UI.Info("%s is already stopped", name)
				return nil
			}
			return a.stopVM(ctx, name, "Stopping "+name)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "the project file (default: .limabean.yaml here or in a parent directory)")
	cmd.Flags().DurationVar(&idle, "idle", 0, "only stop VMs unused for this long, e.g. 2h; without a name, every running VM is checked")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "with --idle, show which VMs would stop without stopping them")
	return cmd
}

// targetVM is the VM a command acts on: the one named, else the project's
// (from its .limabean.yaml), else one picked in a terminal ("VM to verb").
// It returns "" when the project has no VM yet, after saying so.
func (a *App) targetVM(ctx context.Context, args []string, file, command, verb string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	_, f, err := findProject(file)
	switch {
	case err != nil:
		return "", err
	case f != nil:
		if _, ok, err := a.Lima.Get(ctx, f.Name); err != nil {
			return "", err
		} else if !ok {
			a.UI.Info("%s has no VM yet; limabean up creates it", f.Name)
			return "", nil
		}
		return f.Name, nil
	case !a.UI.Prompting:
		return "", usageErr("no .limabean.yaml here or in a parent directory; name the VM: limabean %s <name>", command)
	}
	return a.vmArg(ctx, args, verb)
}
