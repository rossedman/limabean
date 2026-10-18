package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newLogsCmd(a *App) *cobra.Command {
	var file string
	var lines int
	var follow, setup, start, limaLog bool
	cmd := &cobra.Command{
		Use:   "logs [name]",
		Short: "Show a VM's provisioning log, or its project's setup and start output",
		Long: `Show what happened in a VM. Without flags, logs shows the end of its
provisioning log (cloud-init's output: packages, packs, every boot's
scripts), the place to look when creating a VM fails or hangs.

  --setup   the output of the project's setup, from its last run
  --start   the output of the project's start commands, still running or not
  --lima    Lima's own log on your Mac, for a VM that won't boot

-n sets how many lines (default 100), and -f keeps following new ones
until you press Ctrl-C. Without a name, logs uses the project's VM (from
.limabean.yaml here or in a parent directory), or asks which VM in a
terminal.

A locked VM's user has no sudo, so its provisioning log can't be read from
inside it; --lima still works.`,
		Example: `  # Why did creating the VM fail?
  limabean logs dev

  # Follow the project's dev server, started by start: in .limabean.yaml
  limabean logs --start -f

  # The last setup run, all of it
  limabean logs --setup -n 1000`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			chosen := 0
			for _, b := range []bool{setup, start, limaLog} {
				if b {
					chosen++
				}
			}
			if chosen > 1 {
				return errors.New("pick one of --setup, --start and --lima")
			}
			if lines < 1 {
				return errors.New("-n needs a number of lines, 1 or more")
			}
			name, err := a.targetVM(ctx, args, file, "logs", "read the logs of")
			if err != nil || name == "" {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if limaLog {
				if follow {
					return errors.New("--lima shows a file on your Mac; follow it there: tail -f " + filepath.Join(inst.Dir, "ha.stderr.log"))
				}
				return a.limaLogTail(inst.Dir, lines)
			}
			if !inst.Running() {
				return fmt.Errorf("%s is stopped; start it with limabean up %s, or see Lima's log with --lima", name, name)
			}
			path, sudo := "/var/log/cloud-init-output.log", "sudo -n "
			switch {
			case setup:
				path, sudo = vmLogDir+"/setup.log", ""
			case start:
				path, sudo = vmLogDir+"/start.log", ""
			}
			tail := sudo + "tail -n " + strconv.Itoa(lines)
			if follow {
				return a.execSSH(inst, tail+" -F \""+path+"\"")
			}
			out, err := a.Lima.Run(ctx, inst, nil, tail+" \""+path+"\"")
			if err != nil {
				switch {
				case sudo != "":
					return fmt.Errorf("can't read %s's provisioning log (a locked VM has no sudo): %w; try --lima", name, err)
				case setup:
					return fmt.Errorf("%s has no setup log yet: setup runs when limabean up creates the VM, or with --setup", name)
				default:
					return fmt.Errorf("%s has no start log yet: start runs after limabean up when .limabean.yaml has start:", name)
				}
			}
			_, err = a.UI.Out.Write(out)
			return err
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "the project file (default: .limabean.yaml here or in a parent directory)")
	cmd.Flags().IntVarP(&lines, "lines", "n", 100, "how many lines from the end")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing new lines until Ctrl-C")
	cmd.Flags().BoolVar(&setup, "setup", false, "the project's setup output")
	cmd.Flags().BoolVar(&start, "start", false, "the project's start output")
	cmd.Flags().BoolVar(&limaLog, "lima", false, "Lima's log on your Mac")
	return cmd
}

// limaLogTail prints the end of Lima's host-agent log for a VM, which is
// on the Mac, in the VM's own Lima directory.
func (a *App) limaLogTail(dir string, lines int) error {
	b, err := os.ReadFile(filepath.Join(dir, "ha.stderr.log"))
	if err != nil {
		return fmt.Errorf("no Lima log: %w", err)
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	var out bytes.Buffer
	for _, l := range all {
		out.WriteString(l + "\n")
	}
	_, err = a.UI.Out.Write(out.Bytes())
	return err
}
