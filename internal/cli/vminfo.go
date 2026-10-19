package cli

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
)

// vmInfoPath is where a VM finds out about itself: limabean writes it, and
// limabean-info (templates/files/) prints it. It lives in the user's home,
// not /etc, because a locked VM has no sudo for later writes.
const vmInfoPath = "$HOME/.config/limabean/vm.json"

// vmInfo is what a VM, and an agent working in it, can read about itself.
// Secrets and the network aren't in it: limabean-info reads them live, the
// secrets' names from secrets.env and the policy from the read-only mount.
type vmInfo struct {
	Name      string            `json:"name"`
	Env       string            `json:"env"`
	Packs     []string          `json:"packs"`
	Tools     map[string]string `json:"tools,omitempty"`
	Apt       []string          `json:"apt,omitempty"`
	Project   *vmInfoProject    `json:"project,omitempty"`
	Ports     string            `json:"ports"`
	Logs      map[string]string `json:"logs"`
	WrittenBy string            `json:"written_by"`
	Written   string            `json:"written"`
}

type vmInfoProject struct {
	Repo  string   `json:"repo,omitempty"`
	Clone string   `json:"clone,omitempty"`
	Ports []int    `json:"ports,omitempty"`
	Vars  []string `json:"vars,omitempty"`
	Setup string   `json:"setup,omitempty"`
	Start string   `json:"start,omitempty"`
}

// vmInfoFor gathers a VM's info from what limabean keeps on the Mac, plus
// its project file and clone when it's a project's VM (f may be nil).
func (a *App) vmInfoFor(name string, f *project.File, repo, clone string) vmInfo {
	packs, _ := a.packsOf(name)
	tools := a.toolsOf(name)
	info := vmInfo{
		Name:  name,
		Env:   firstNonEmpty(a.envOf(name), "scratch"),
		Packs: append([]string{"core"}, packs...),
		Tools: tools.Tools,
		Apt:   tools.Apt,
		Ports: "A port listening in this VM is http://localhost:<port> on the Mac.",
		Logs: map[string]string{
			"provision": "/var/log/cloud-init-output.log",
			"network":   "journalctl -u limabean-network",
		},
		WrittenBy: "limabean " + a.Version,
		Written:   time.Now().UTC().Format(time.RFC3339),
	}
	if f != nil {
		p := &vmInfoProject{Repo: repo, Clone: clone, Ports: f.Ports, Setup: f.Setup, Start: f.Start}
		p.Vars = slices.Sorted(maps.Keys(f.Vars))
		info.Project = p
		if f.Setup != "" {
			info.Logs["setup"] = "~/.local/state/limabean/setup.log"
		}
		if f.Start != "" {
			info.Logs["start"] = "~/.local/state/limabean/start.log"
		}
	}
	return info
}

// writeVMInfo writes a VM's info into it. The file only describes the VM,
// so failing to write it warns rather than stopping the command.
func (a *App) writeVMInfo(ctx context.Context, inst lima.Instance, f *project.File, repo, clone string) {
	b, _ := json.MarshalIndent(a.vmInfoFor(inst.Name, f, repo, clone), "", "  ")
	cmd := `f=` + vmInfoPath + ` && mkdir -p "${f%/*}" && cat > "$f.new" && mv "$f.new" "$f"`
	if _, err := a.Lima.Run(ctx, inst, strings.NewReader(string(b)+"\n"), cmd); err != nil {
		a.UI.Warn("Couldn't write %s's info file (limabean-info): %v", inst.Name, err)
	}
}
