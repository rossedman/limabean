package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAutostopOnCmd(a *App) *cobra.Command {
	var after time.Duration
	cmd := &cobra.Command{
		Use:   "on",
		Short: "Stop VMs that nobody has used for a while",
		Long: `Install a launchd agent that runs "limabean stop --idle <after>" every 10
minutes while you're logged in. Running it again changes the limit.

The agent runs the limabean on your PATH (so upgrades are picked up) and
finds limactl where it is now. Its output goes to
~/Library/Logs/limabean-autostop.log.`,
		Example: `  limabean autostop on
  limabean autostop on --after 45m`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if after < autostopInterval {
				return fmt.Errorf("--after must be at least %s, how often VMs are checked", shortDuration(autostopInterval))
			}
			limactl, err := lookPath("limactl")
			if err != nil {
				return errors.New("limactl isn't on your PATH (brew install lima)")
			}
			program := []string{a.selfPath(), "stop", "--idle", shortDuration(after), "--plain"}
			if a.configPath != "" {
				cfg, err := filepath.Abs(a.configPath)
				if err != nil {
					return err
				}
				program = append(program, "--config", cfg)
			}
			// launchd starts agents with a bare PATH: add where limactl and ssh are.
			dirs := []string{filepath.Dir(limactl)}
			if ssh, err := lookPath("ssh"); err == nil {
				dirs = append(dirs, filepath.Dir(ssh))
			}
			for _, d := range []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin"} {
				if !slices.Contains(dirs, d) {
					dirs = append(dirs, d)
				}
			}

			plist := autostopPlist()
			if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(autostopLog()), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(plist, renderPlist(program, strings.Join(dirs, ":"), autostopInterval), 0o644); err != nil {
				return err
			}
			// Replace an agent that's already loaded, so a new limit applies.
			_, _ = a.launchctl(ctx, "bootout", launchdTarget())
			if out, err := a.launchctl(ctx, "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), plist); err != nil {
				return fmt.Errorf("launchctl bootstrap: %v: %s", err, strings.TrimSpace(string(out)))
			}
			a.UI.OK("VMs nobody has used for %s are stopped (checked every %s)", shortDuration(after), shortDuration(autostopInterval))
			a.UI.Hint("Log: "+shortPath(autostopLog()), "Turn it off with: limabean autostop off")
			return nil
		},
	}
	cmd.Flags().DurationVar(&after, "after", 2*time.Hour, "stop a VM once nobody has used it for this long")
	return cmd
}

// selfPath is the limabean the agent should run: the one on PATH when it's
// this binary (a stable Homebrew or go install link), else this binary.
func (a *App) selfPath() string {
	exe, _ := os.Executable()
	if onPath, err := lookPath("limabean"); err == nil {
		if same(onPath, exe) {
			return onPath
		}
	}
	return exe
}

// same reports whether two paths are the same file.
func same(p, q string) bool {
	a, err1 := os.Stat(p)
	b, err2 := os.Stat(q)
	return err1 == nil && err2 == nil && os.SameFile(a, b)
}
