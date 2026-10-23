package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
)

// newPortsCmd builds `limabean ports`, which lists the ports a VM forwards
// to the Mac.
func newPortsCmd(a *App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "ports [name]",
		Short: "List the ports a VM forwards to your Mac",
		Long: `List the TCP ports programs in a VM are listening on, and whether each one
reaches your Mac. Lima forwards any port a program opens on localhost or on
all addresses in the VM (a dev server, or a container started with -p) to
the same port on the Mac's localhost, so http://localhost:3000 on the Mac
reaches port 3000 in the VM.

A port shows as not forwarded when nothing answers on the Mac's side, for
example because a program on the Mac already uses that port. Ports a
program opens on another address, like 127.0.0.53, aren't forwarded and
aren't listed, and neither is SSH.

Leave out the name to pick a VM from a list.`,
		Example: `  limabean ports dev

  # Open one in the browser
  limabean open dev 3000

  # For scripts
  limabean ports dev --json | jq '.[] | select(.forwarded) | .port'`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := a.projectVMArg(ctx, args, "list ports of")
			if err != nil {
				return err
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.Running() {
				return fmt.Errorf("%s is stopped, so nothing is forwarded (start it with: limabean up %s)", name, name)
			}
			list, err := a.ports(ctx, inst)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(a.UI.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(list)
			}
			if len(list) == 0 {
				a.UI.Info("Nothing in %s is listening on a forwarded port", name)
				return nil
			}
			rows := make([][]string, 0, len(list))
			for _, p := range list {
				mac := "not forwarded"
				if p.Forwarded {
					mac = p.URL
				}
				rows = append(rows, []string{strconv.Itoa(p.Port), firstNonEmpty(p.Process, "-"), mac})
			}
			a.UI.Table([]string{"PORT", "PROCESS", "ON YOUR MAC"}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print a JSON array, for scripts")
	return cmd
}

// vmPort is a port a program in a VM listens on.
type vmPort struct {
	Port      int    `json:"port"`
	Process   string `json:"process"` // "" when it isn't the VM user's
	Forwarded bool   `json:"forwarded"`
	URL       string `json:"url"`
}

// ports lists the TCP ports in a running VM that Lima's default rule
// forwards, and checks which of them answer on the Mac.
func (a *App) ports(ctx context.Context, inst lima.Instance) ([]vmPort, error) {
	out, err := a.Lima.Run(ctx, inst, nil, "ss -ltnpH")
	if err != nil {
		return nil, fmt.Errorf("listing ports in %s: %w", inst.Name, err)
	}
	list := parseListening(string(out))
	for i := range list {
		list[i].URL = fmt.Sprintf("http://localhost:%d", list[i].Port)
		list[i].Forwarded = a.macListens(list[i].Port)
	}
	return list, nil
}

// parseListening reads `ss -ltnpH` output, keeping the ports Lima forwards
// by default: those bound to localhost or to every address, except SSH.
// A port bound on both IPv4 and IPv6 is listed once.
func parseListening(out string) []vmPort {
	var list []vmPort
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		local := f[3]
		i := strings.LastIndex(local, ":")
		if i < 0 {
			continue
		}
		port, err := strconv.Atoi(local[i+1:])
		if err != nil || port == 22 {
			continue
		}
		switch strings.Trim(local[:i], "[]") {
		case "127.0.0.1", "0.0.0.0", "::", "::1", "*":
		default:
			continue
		}
		proc := ""
		if len(f) > 5 {
			proc = processName(f[len(f)-1])
		}
		if j := slices.IndexFunc(list, func(p vmPort) bool { return p.Port == port }); j >= 0 {
			list[j].Process = firstNonEmpty(list[j].Process, proc)
			continue
		}
		list = append(list, vmPort{Port: port, Process: proc})
	}
	slices.SortFunc(list, func(x, y vmPort) int { return x.Port - y.Port })
	return list
}

// processName pulls the first program name out of ss's
// users:(("node",pid=1234,fd=20)) column.
func processName(users string) string {
	_, rest, ok := strings.Cut(users, `(("`)
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, `"`)
	return name
}

// macListens reports whether something answers on the Mac's localhost:port.
func macListens(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// openURL opens a URL in the Mac's default browser.
func openURL(ctx context.Context, url string) error {
	return exec.CommandContext(ctx, "open", url).Run()
}
