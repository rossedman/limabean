package cli

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAutostopCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autostop",
		Short: "Stop VMs automatically when nobody is using them",
		Long: `Stop idle VMs in the background, so forgotten ones don't hold on to memory
and battery, and delete VMs made with "new --ttl" once they expire.
"autostop on" installs a launchd agent in your login session that runs
"limabean autostop run" every 10 minutes; see "limabean stop --help" for
what counts as in use.

A stopped VM keeps its disk. "limabean ssh", "code" and "start" bring it
back, and Claude Desktop lists it again once it runs.`,
		Example: `  # Stop VMs nobody has used for two hours
  limabean autostop on

  # A longer grace period
  limabean autostop on --after 4h

  limabean autostop status
  limabean autostop off`,
	}
	cmd.AddCommand(newAutostopOnCmd(a), newAutostopOffCmd(a), newAutostopStatusCmd(a), newAutostopRunCmd(a))
	return cmd
}

const (
	autostopLabel    = "io.github.rossedman.limabean.autostop"
	autostopInterval = 10 * time.Minute
)

// autostopPlist is where the launchd agent is installed.
func autostopPlist() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", autostopLabel+".plist")
}

// autostopLog is where the agent's output goes.
func autostopLog() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "limabean-autostop.log")
}

// launchdTarget names the agent in the user's login session.
func launchdTarget() string { return fmt.Sprintf("gui/%d/%s", os.Getuid(), autostopLabel) }

// runLaunchctl runs launchctl, returning its output for error messages.
func runLaunchctl(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
}

// renderPlist writes the launchd agent that runs program every interval.
func renderPlist(program []string, path string, interval time.Duration) []byte {
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	var args strings.Builder
	for _, p := range program {
		fmt.Fprintf(&args, "\n    <string>%s</string>", esc(p))
	}
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<!-- Written by "limabean autostop on"; remove with "limabean autostop off". -->
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>%s
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>%s</string>
  </dict>
  <key>StartInterval</key>
  <integer>%d</integer>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, autostopLabel, args.String(), esc(path), int(interval.Seconds()), esc(autostopLog()), esc(autostopLog()))
}

// autostopExpires reports whether the autostop agent deletes expired VMs
// (agents installed before --ttl only stop idle ones).
func (a *App) autostopExpires(ctx context.Context) bool {
	plist, err := os.ReadFile(autostopPlist())
	if err != nil || !bytes.Contains(plist, []byte("<string>run</string>")) {
		return false
	}
	_, err = a.launchctl(ctx, "print", launchdTarget())
	return err == nil
}

var plistIdleRE = regexp.MustCompile(`<string>--idle</string>\s*<string>([^<]+)</string>`)

// autostopAfter reads the idle limit from an installed agent ("" if none).
func autostopAfter(plist []byte) string {
	if m := plistIdleRE.FindSubmatch(plist); m != nil {
		return string(m[1])
	}
	return ""
}

// shortDuration formats a duration the way it's typed: 2h, 90m, 1h30m.
func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
