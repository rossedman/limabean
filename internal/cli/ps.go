package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/ui"
)

// psPort is a port in ps: one something listens on, or one the project
// file lists that nothing does.
type psPort struct {
	vmPort
	Listening bool `json:"listening"`
}

// psEntry is ps --json.
type psEntry struct {
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Status   string   `json:"status"`   // Running, Stopped, or none: not created yet
	Start    string   `json:"start"`    // running, stopped, none, unapproved, or unknown
	Ports    []psPort `json:"ports"`    // what listens, and the file's ports that don't
	Changes  []string `json:"changes"`  // what up would change
	Widening []string `json:"widening"` // network changes only the user can make
}

// newPsCmd builds `limabean ps`, which shows the project's VM: status,
// start, ports and pending changes.
func newPsCmd(a *App) *cobra.Command {
	var file string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "Show the project's VM: status, start, ports and pending changes",
		Long: `Show the state of the project you're in, from its .limabean.yaml (or
.limabean.yml) here or in a parent directory:

  - its VM: running, stopped, or not created yet
  - its start commands: running, stopped, or changed and not approved yet
  - the ports programs in it listen on, and whether each reaches your Mac,
    plus any port the file lists that nothing listens on
  - what "limabean up" would change to match the file, and the network
    changes only you can make

ps changes nothing. For every VM at once, use limabean ls.`,
		Example: `  cd ~/src/myapp && limabean ps

  # For scripts
  limabean ps --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			path, f, err := findProject(file)
			if err != nil {
				return err
			}
			if f == nil {
				return notFoundErr("no .limabean.yaml here or in a parent directory; for every VM, use limabean ls")
			}
			e := psEntry{Name: f.Name, File: shortPath(path), Status: "none", Start: "none", Ports: []psPort{}, Changes: []string{}, Widening: []string{}}
			inst, ok, err := a.Lima.Get(ctx, f.Name)
			if err != nil {
				return err
			}
			if ok {
				e.Status = inst.Status
				own, _ := a.projectSecrets(f.Secrets)
				c, err := a.upDiff(inst, f, own)
				if err != nil {
					return err
				}
				e.Changes = append(e.Changes, c.lines...)
				e.Widening = append(e.Widening, c.widening...)
				switch {
				case f.Start == "":
				case f.Start != a.approvedStart(f.Name):
					e.Start = "unapproved"
				case !inst.Running():
					e.Start = "stopped"
				default:
					e.Start = a.startState(ctx, inst)
				}
				if inst.Running() {
					list, err := a.ports(ctx, inst)
					if err != nil {
						return err
					}
					for _, p := range list {
						e.Ports = append(e.Ports, psPort{vmPort: p, Listening: true})
					}
				}
			}
			for _, p := range f.Ports {
				if !slices.ContainsFunc(e.Ports, func(v psPort) bool { return v.Port == p }) {
					e.Ports = append(e.Ports, psPort{vmPort: vmPort{Port: p, URL: fmt.Sprintf("http://localhost:%d", p)}})
				}
			}
			if asJSON {
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(e)
			}
			a.psShow(e)
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "the project file (default: .limabean.yaml here or in a parent directory)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print it as JSON, for scripts")
	return cmd
}

// psShow prints ps for people, on stdout: the report is the result.
func (a *App) psShow(e psEntry) {
	defer func(u *ui.UI) { a.UI = u }(a.UI)
	a.UI = a.UI.Report()
	a.UI.Section(e.Name, "from "+e.File)
	vm := e.Status
	if e.Status == "none" {
		vm = "not created yet: limabean up"
	}
	start := map[string]string{
		"running":    "running (limabean logs --start -f)",
		"stopped":    "not running; limabean up starts it",
		"none":       "none in the file",
		"unapproved": "changed since you approved it; limabean up shows it",
		"unknown":    "couldn't tell",
	}[e.Start]
	rows := [][]string{{"VM", vm}, {"start", start}}
	for _, p := range e.Ports {
		var state string
		switch {
		case e.Status != "Running":
			state = "VM not running"
		case !p.Listening:
			state = "nothing listening"
		case p.Forwarded:
			state = p.URL
		default:
			state = "not forwarded"
		}
		label := strconv.Itoa(p.Port)
		if p.Process != "" {
			label += " " + p.Process
		}
		rows = append(rows, []string{"port " + label, state})
	}
	file := "up to date"
	if len(e.Changes) > 0 {
		file = fmt.Sprintf("%d change%s for limabean up:", len(e.Changes), plural(len(e.Changes)))
	}
	rows = append(rows, []string{"file", file})
	width := 0
	for _, r := range rows {
		width = max(width, len(r[0]))
	}
	var lines []string
	for _, r := range rows {
		lines = append(lines, r[0]+strings.Repeat(" ", width-len(r[0])+2)+r[1])
	}
	for _, c := range e.Changes {
		lines = append(lines, "  "+c)
	}
	a.UI.Hint(lines...)
	if len(e.Widening) > 0 {
		a.UI.Warn("The file allows more than %s does now, which only you can change:", e.Name)
		a.UI.Hint(e.Widening...)
	}
}

// plural returns "s" unless n is 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
