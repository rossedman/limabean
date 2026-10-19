package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/atomic"
	"github.com/rossedman/limabean/internal/pins"
)

// limabeanRepo is where releases are published.
const limabeanRepo = "rossedman/limabean"

// releaseRE is a release version, as tagged: v0.3.0, v0.3.0-rc1. Dev and
// go-install builds don't match, so they never check.
var releaseRE = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

// updateState is what the last check found, kept in the state directory.
type updateState struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest,omitempty"`
}

func (a *App) updateFile() string { return filepath.Join(a.stateDir, "update.json") }

func (a *App) readUpdateState() updateState {
	var s updateState
	if b, err := os.ReadFile(a.updateFile()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func (a *App) saveUpdateState(s updateState) error {
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(s)
	return atomic.WriteFile(a.updateFile(), b, 0o644)
}

// updateNotice runs after a command: in a terminal, it says when a newer
// limabean is out, from the last check, and starts a check in the
// background once a day. It never slows a command or prints where a script
// or Claude reads, and UPDATE_CHECK=no turns it off.
func (a *App) updateNotice(cmd *cobra.Command) {
	if a.Cfg == nil || !a.Cfg.UpdateCheck || !releaseRE.MatchString(a.Version) || a.mcpMode || !a.UI.Prompting || a.UI.Quiet {
		return
	}
	if f := cmd.Flags().Lookup("json"); f != nil && f.Changed {
		return
	}
	switch cmd.Name() {
	case "update-check", "mcp", "completion", "__complete":
		return
	}
	s := a.readUpdateState()
	if releaseRE.MatchString(s.Latest) && pins.Newer(s.Latest, a.Version) {
		a.UI.Hint("limabean " + strings.TrimPrefix(s.Latest, "v") + " is out (this is " + strings.TrimPrefix(a.Version, "v") + "): brew upgrade limabean")
	}
	if time.Since(s.Checked) < 24*time.Hour {
		return
	}
	// Recorded first, so a check that fails isn't retried on every command.
	s.Checked = time.Now()
	if err := a.saveUpdateState(s); err == nil {
		_ = a.spawn("update-check")
	}
}
