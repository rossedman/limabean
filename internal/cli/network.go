package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/lima"
)

func newNetworkCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Limit which hosts a VM can reach, and lock it down",
		Long: `Give a VM an allowlist of domains it may reach; everything else is
blocked, for the VM and its containers. A domain allows its subdomains, and
presets name common sets:

` + presetHelp() + `

Locking a VM also takes sudo and Docker away from its user. Without that,
anything running in the VM, an agent included, could become root and turn
the allowlist off; with it, the allowlist holds. Locked VMs suit agents
that should only reach what their task needs.

The policy is kept on your Mac (~/.config/limabean/network/<vm>/) and
mounted read-only into the VM, so nothing inside can change it. Changes
apply to a running VM at once, and to a stopped one when it starts.
Disallowed names don't resolve, so data can't leave through DNS either.
VMs created before this feature need a rebuild first.`,
		Example: `  # A sandbox that can reach GitHub and PyPI, and can't undo that
  limabean new agent --allow github --allow python --locked

  limabean network ls agent
  limabean network allow agent registry.npmjs.org
  limabean network rm agent python
  limabean network open agent
  limabean network unlock agent`,
	}
	cmd.AddCommand(newNetworkLsCmd(a), newNetworkAllowCmd(a), newNetworkRmCmd(a), newNetworkOpenCmd(a), newNetworkLockCmd(a), newNetworkUnlockCmd(a))
	return cmd
}

// netPresets are names for sets of domains. Each domain allows its
// subdomains too.
var netPresets = map[string][]string{
	"claude": {"anthropic.com", "claude.ai", "claude.com"},
	"docker": {"docker.io", "docker.com"},
	"github": {"github.com", "githubusercontent.com", "githubassets.com", "ghcr.io"},
	"java":   {"maven.org", "apache.org", "gradle.org", "gradle-dn.com"},
	"go":     {"go.dev", "golang.org", "proxy.golang.org", "sum.golang.org"},
	"npm":    {"npmjs.org", "npmjs.com", "yarnpkg.com"},
	"python": {"pypi.org", "pythonhosted.org"},
	"ruby":   {"rubygems.org", "ruby-lang.org"},
	"rust":   {"crates.io", "rust-lang.org"},
	"ubuntu": {"ubuntu.com"},
}

func presetHelp() string {
	names := make([]string, 0, len(netPresets))
	for n := range netPresets {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "  %-8s %s\n", n, strings.Join(netPresets[n], ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

var netDomainRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// expandAllow turns presets and domains into domains.
func expandAllow(items []string) ([]string, error) {
	var out []string
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if domains, ok := netPresets[item]; ok {
			out = append(out, domains...)
			continue
		}
		item = strings.TrimPrefix(item, "*.")
		if !netDomainRE.MatchString(item) {
			names := mapKeys(netPresets)
			sort.Strings(names)
			return nil, fmt.Errorf("%q isn't a domain or a preset (%s)", item, strings.Join(names, ", "))
		}
		out = append(out, item)
	}
	return out, nil
}

func mapKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// netPolicy is a VM's network.conf.
type netPolicy struct {
	Locked bool
	Allow  []string // sorted, unique
}

func (p netPolicy) open() bool { return len(p.Allow) == 0 }

// summary describes the policy in a few words.
func (p netPolicy) summary() string {
	s := "open"
	if !p.open() {
		s = fmt.Sprintf("%d domain(s)", len(p.Allow))
	}
	if p.Locked {
		s += ", locked"
	}
	return s
}

func (a *App) networkDir(vm string) string { return filepath.Join(a.Cfg.Dir(), "network", vm) }

func (a *App) loadPolicy(vm string) (netPolicy, error) {
	var p netPolicy
	f, err := os.Open(filepath.Join(a.networkDir(vm), "network.conf"))
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	} else if err != nil {
		return p, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		switch {
		case len(fields) == 1 && fields[0] == "locked":
			p.Locked = true
		case len(fields) == 2 && fields[0] == "allow":
			p.Allow = append(p.Allow, fields[1])
		}
	}
	return p, sc.Err()
}

func (a *App) savePolicy(vm string, p netPolicy) error {
	slices.Sort(p.Allow)
	p.Allow = slices.Compact(p.Allow)
	dir := a.networkDir(vm)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Network policy for %s, applied by limabean-network in the VM.\n# Change it with: limabean network --help\n", vm)
	if p.Locked {
		b.WriteString("locked\n")
	}
	for _, d := range p.Allow {
		fmt.Fprintf(&b, "allow %s\n", d)
	}
	tmp := filepath.Join(dir, ".network.conf.new")
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "network.conf"))
}

// applyPolicy makes a running VM re-read its policy; a stopped one reads it
// when it starts. A locked VM's user may still run exactly this.
func (a *App) applyPolicy(ctx context.Context, inst lima.Instance) error {
	if !inst.NetworkPolicy {
		return fmt.Errorf("%s was created before network policies; rebuild it first (limabean rebuild %s)", inst.Name, inst.Name)
	}
	if !inst.Running() {
		a.UI.Info("%s is stopped; the policy applies when it starts", inst.Name)
		return nil
	}
	if out, err := a.Lima.Run(ctx, inst, nil, "sudo -n systemctl start limabean-network.service"); err != nil {
		a.UI.Hint("See why with: limabean ssh " + inst.Name + " journalctl -u limabean-network")
		return fmt.Errorf("applying %s's network policy: %w %s", inst.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// changePolicy loads a VM's policy, lets change edit it, saves and applies
// it, and reports the result.
func (a *App) changePolicy(ctx context.Context, vm string, change func(*netPolicy) error) error {
	inst, err := a.vm(ctx, vm)
	if err != nil {
		return err
	}
	if !inst.NetworkPolicy {
		return fmt.Errorf("%s was created before network policies; rebuild it first (limabean rebuild %s)", vm, vm)
	}
	p, err := a.loadPolicy(vm)
	if err != nil {
		return err
	}
	if err := change(&p); err != nil {
		return err
	}
	if err := a.savePolicy(vm, p); err != nil {
		return err
	}
	if err := a.applyPolicy(ctx, inst); err != nil {
		return err
	}
	a.UI.OK("%s's network: %s", a.UI.Code(vm), p.summary())
	return nil
}
