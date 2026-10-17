package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
)

func newStopCmd(a *App) *cobra.Command {
	var idle time.Duration
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running VM",
		Long: `Shut a VM down. Its disk, and everything in ~/code, is kept; start it again
with "limabean start <name>" (or "limabean ssh <name>").

Its Claude Desktop connection is removed while it's stopped (Claude Desktop
shows an error for a VM it can't reach) and added back when limabean starts
it.

Leave out the name to pick a VM from a list.

With --idle, only VMs nobody has used for that long are stopped, and
without a name every running VM is checked. A VM is in use while:
  - a terminal in it (a shell, tmux, limabean ssh) had input or output
  - Claude Desktop, VS Code or Cursor is connected to it
  - its 5-minute load average is 1 or more, e.g. a build or busy containers
  - it started less than that long ago
"limabean autostop on" runs this every few minutes.`,
		Example: `  limabean stop dev

  # Pick the VM from a list
  limabean stop

  # Stop every VM unused for two hours
  limabean stop --idle 2h

  # See which VMs that would stop, and why the others are kept
  limabean stop --idle 2h --dry-run`,
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
			name, err := a.vmArg(ctx, args, "stop")
			if err != nil {
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
	cmd.Flags().DurationVar(&idle, "idle", 0, "only stop VMs unused for this long, e.g. 2h; without a name, every running VM is checked")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "with --idle, show which VMs would stop without stopping them")
	return cmd
}

// stopVM stops a running VM and removes its Claude Desktop entry.
func (a *App) stopVM(ctx context.Context, name, title string) error {
	if err := a.limaStep(ctx, title, func(ctx context.Context) ([]byte, error) {
		return a.Lima.Stop(ctx, name)
	}); err != nil {
		return err
	}
	a.UI.OK("Stopped %s", a.UI.Code(name))
	a.claudeRemove(name)
	return nil
}

// stopIdle stops the named VM, or every running VM, if it has been idle for
// at least limit. It never prompts, so it can run unattended.
func (a *App) stopIdle(ctx context.Context, args []string, limit time.Duration, dryRun bool) error {
	var targets []lima.Instance
	if len(args) > 0 {
		inst, err := a.vm(ctx, args[0])
		if err != nil {
			return err
		}
		targets = []lima.Instance{inst}
	} else {
		list, err := a.vms(ctx)
		if err != nil {
			return err
		}
		targets = list
	}
	failed, running := false, 0
	for _, inst := range targets {
		if !inst.Running() {
			if len(args) > 0 {
				a.UI.Info("%s is already stopped", inst.Name)
			}
			continue
		}
		running++
		st, err := a.idleState(ctx, inst)
		switch {
		case err != nil:
			a.UI.Warn("%s: couldn't tell whether it's in use, so it's left running: %v", inst.Name, err)
			failed = true
		case st.busy != "":
			a.UI.Info("%s is in use: %s", inst.Name, st.busy)
		case st.idle < limit:
			a.UI.Info("%s was used %s ago", inst.Name, roughDuration(st.idle))
		case dryRun:
			a.UI.Info("%s has been idle for %s, so it would stop", inst.Name, roughDuration(st.idle))
		default:
			if err := a.stopVM(ctx, inst.Name, fmt.Sprintf("Stopping %s, idle for %s", inst.Name, roughDuration(st.idle))); err != nil {
				a.UI.Warn("%v", err)
				failed = true
			}
		}
	}
	if running == 0 && len(args) == 0 {
		a.UI.Info("No VMs are running")
	}
	if failed {
		return errSilent
	}
	return nil
}

// vmActivity is what idleScript found in a VM.
type vmActivity struct {
	idle time.Duration // since the last sign of use
	busy string        // why the VM is in use right now, if it is
}

// idleScript prints the VM's clock, the time it was last used and, if
// something is using it right now, why. Times come from the VM's own clock,
// so the Mac's doesn't need to agree with it. A terminal's access and
// modification times are when it last had input and output, as for w(1).
const idleScript = `now=$(date +%s)
last=$((now - $(cut -d. -f1 /proc/uptime)))
for t in /dev/pts/[0-9]*; do
  [ -O "$t" ] || continue
  for ts in $(stat -c '%X %Y' "$t" 2>/dev/null); do
    [ "$ts" -gt "$last" ] && last=$ts
  done
done
busy=
if pgrep -u "$(id -u)" -f '[.]claude/remote' >/dev/null; then busy="Claude is connected"
elif pgrep -u "$(id -u)" -f '[.](vscode|cursor|windsurf)(-insiders)?-server' >/dev/null; then busy="an editor is connected"
elif awk '{ exit !($2 >= 1) }' /proc/loadavg; then busy="load average $(cut -d' ' -f2 /proc/loadavg)"
fi
echo "$now $last $busy"`

// idleState runs idleScript in a running VM.
func (a *App) idleState(ctx context.Context, inst lima.Instance) (vmActivity, error) {
	out, err := a.Lima.Run(ctx, inst, strings.NewReader(idleScript), "sh -s")
	if err != nil {
		return vmActivity{}, err
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return vmActivity{}, fmt.Errorf("unexpected output %q", strings.TrimSpace(string(out)))
	}
	now, err1 := strconv.ParseInt(f[0], 10, 64)
	last, err2 := strconv.ParseInt(f[1], 10, 64)
	if err := errors.Join(err1, err2); err != nil {
		return vmActivity{}, fmt.Errorf("unexpected output %q", strings.TrimSpace(string(out)))
	}
	return vmActivity{idle: time.Duration(max(now-last, 0)) * time.Second, busy: strings.Join(f[2:], " ")}, nil
}

// roughDuration shows a duration to the minute: "under a minute", "45m",
// "2h05m".
func roughDuration(d time.Duration) string {
	m := int(d / time.Minute)
	switch {
	case m < 1:
		return "under a minute"
	case m < 60:
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh%02dm", m/60, m%60)
}
