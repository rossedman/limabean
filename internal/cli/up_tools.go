package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rossedman/limabean/internal/atomic"
	"github.com/rossedman/limabean/internal/project"
)

// projectTools are a project file's own mise tools and apt packages. They
// go into a new VM next to its packs' lists (90-project.toml sorts after
// every pack's 50-<name>.toml; _project.list can't be a pack's name), and
// the same boot scripts install them, so VMs copied from an image get them
// too.
type projectTools struct {
	Tools map[string]string `json:"tools,omitempty"`
	Apt   []string          `json:"apt,omitempty"`
}

// toolsOfFile is a project file's tools, apt packages sorted.
func toolsOfFile(f *project.File) projectTools {
	t := projectTools{Tools: f.Tools, Apt: slices.Compact(slices.Sorted(slices.Values(f.Apt)))}
	if len(t.Tools) == 0 {
		t.Tools = nil
	}
	if len(t.Apt) == 0 {
		t.Apt = nil
	}
	return t
}

// empty reports whether the project asks for no tools and no apt packages.
func (t projectTools) empty() bool { return len(t.Tools) == 0 && len(t.Apt) == 0 }

// toolList describes the tools for plans: "go 1.23, terraform 1.9.8".
func (t projectTools) toolList() string {
	if len(t.Tools) == 0 {
		return "none"
	}
	var parts []string
	for _, name := range slices.Sorted(maps.Keys(t.Tools)) {
		parts = append(parts, name+" "+t.Tools[name])
	}
	return strings.Join(parts, ", ")
}

// aptList describes the apt packages for plans: "git, jq", or "none".
func (t projectTools) aptList() string {
	if len(t.Apt) == 0 {
		return "none"
	}
	return strings.Join(t.Apt, ", ")
}

// provision returns the limactl --set expressions that write the lists
// into a new VM. project.Load allows only names and versions, so the TOML
// can't be broken out of.
func (t projectTools) provision() []string {
	var sets []string
	if len(t.Tools) > 0 {
		var b strings.Builder
		b.WriteString("# From tools: in the project's .limabean.yaml, written by limabean.\n[tools]\n")
		for _, name := range slices.Sorted(maps.Keys(t.Tools)) {
			fmt.Fprintf(&b, "%q = %q\n", name, t.Tools[name])
		}
		sets = append(sets, provisionData("/etc/limabean/mise/90-project.toml", b.String()))
	}
	if len(t.Apt) > 0 {
		sets = append(sets, provisionData("/etc/limabean/apt/_project.list", strings.Join(t.Apt, "\n")+"\n"))
	}
	return sets
}

// toolsPlan shows a project's tools for up's plan.
func toolsPlan(t projectTools) []string {
	var plan []string
	if len(t.Tools) > 0 {
		plan = append(plan, "tools: "+t.toolList())
	}
	if len(t.Apt) > 0 {
		plan = append(plan, "apt: "+t.aptList())
	}
	return plan
}

// toolsChanges lists what differs between the tools a VM was built with
// and the file's. Either needs a rebuild: they're installed at first boot.
func toolsChanges(have, want projectTools) []string {
	var lines []string
	if !maps.Equal(have.Tools, want.Tools) {
		lines = append(lines, fmt.Sprintf("tools: %s → %s (rebuild, keeping ~/code)", have.toolList(), want.toolList()))
	}
	if !slices.Equal(have.Apt, want.Apt) {
		lines = append(lines, fmt.Sprintf("apt: %s → %s (rebuild, keeping ~/code)", have.aptList(), want.aptList()))
	}
	return lines
}

// toolsState is the file on the Mac that remembers the project tools a VM
// was built with.
func (a *App) toolsState(vm string) string {
	return filepath.Join(a.stateDir, vm+".tools")
}

// toolsOf returns the project tools a VM was built with: none for VMs
// made without, or before, any.
func (a *App) toolsOf(vm string) projectTools {
	var t projectTools
	if b, err := os.ReadFile(a.toolsState(vm)); err == nil {
		_ = json.Unmarshal(b, &t)
	}
	return t
}

// saveTools remembers a VM's project tools, so rebuild keeps them and up
// can tell when they change.
func (a *App) saveTools(vm string, t projectTools) error {
	if t.empty() {
		if err := os.Remove(a.toolsState(vm)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(t)
	return atomic.WriteFile(a.toolsState(vm), b, 0o644)
}
