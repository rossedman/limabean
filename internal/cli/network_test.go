package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandAllow(t *testing.T) {
	got, err := expandAllow([]string{"GitHub", "*.Example.com", "registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]string{}, netPresets["github"]...), "example.com", "registry.npmjs.org")
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, bad := range []string{"localhost", "https://pypi.org", "a..b", "-x.com", "x.com/path", ""} {
		if _, err := expandAllow([]string{bad}); err == nil || !strings.Contains(err.Error(), "isn't a domain or a preset") {
			t.Errorf("expandAllow(%q): %v", bad, err)
		}
	}
}

func TestNewWithNetworkPolicy(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	out := h.must("", "new", "agent", "--allow", "github", "--allow", "pypi.org", "--locked")
	if !strings.Contains(out, "Network: 5 domain(s), locked") {
		t.Fatalf("new --allow --locked:\n%s", out)
	}
	conf := h.read(filepath.Join("network", "agent", "network.conf"))
	for _, want := range []string{"locked\n", "allow github.com\n", "allow ghcr.io\n", "allow pypi.org\n"} {
		if !strings.Contains(conf, want) {
			t.Errorf("network.conf missing %q:\n%s", want, conf)
		}
	}
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(calls, "--param networkDir="+filepath.Join(h.dir, "network", "agent")) {
		t.Errorf("new should mount the policy directory: %v", h.world.calls)
	}
	// The policy is applied once provisioning has finished, not before.
	if start, apply := strings.Index(calls, "limactl start --name=agent"), strings.Index(calls, "systemctl start limabean-network"); apply < start {
		t.Errorf("the policy should apply after the VM is created: %v", h.world.calls)
	}

	var vms []lsEntry
	if err := json.Unmarshal([]byte(h.must("", "ls", "--json")), &vms); err != nil || vms[0].Network != "5 domain(s), locked" {
		t.Fatalf("ls --json network: %v %+v", err, vms)
	}
	if out := h.must("", "ls"); !strings.Contains(out, "NETWORK") || !strings.Contains(out, "locked") {
		t.Errorf("ls:\n%s", out)
	}

	// A rebuild applies the kept policy to the new VM.
	h.world.calls = nil
	h.must("", "rebuild", "agent", "--force")
	if !h.world.called("ssh -F /lima/agent/ssh.config -o BatchMode=yes lima-agent sudo -n systemctl start limabean-network.service") {
		t.Errorf("rebuild should apply the policy again: %v", h.world.calls)
	}
	// rm forgets it.
	h.must("", "rm", "agent", "--force")
	if _, err := os.Stat(filepath.Join(h.dir, "network", "agent")); !os.IsNotExist(err) {
		t.Errorf("rm left the network policy: %v", err)
	}
	if _, err := h.run("", "new", "x", "--allow", "not a domain"); err == nil {
		t.Error("a bad --allow should fail")
	}
}

func TestNetworkCommands(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	conf := func() string { return h.read(filepath.Join("network", "dev", "network.conf")) }

	if out := h.must("", "network", "ls", "dev"); !strings.Contains(out, "Open: it can reach any host") {
		t.Fatalf("ls of an open VM:\n%s", out)
	}
	out := h.must("", "network", "allow", "dev", "python", "example.com")
	if !strings.Contains(out, "dev's network: 3 domain(s)") || !h.world.called("ssh -F /lima/dev/ssh.config -o BatchMode=yes lima-dev sudo -n systemctl start limabean-network.service") {
		t.Fatalf("allow:\n%s\n%v", out, h.world.calls)
	}
	h.must("", "network", "allow", "dev", "example.com") // already there: no duplicate
	if strings.Count(conf(), "allow example.com") != 1 {
		t.Errorf("duplicate entries:\n%s", conf())
	}
	if out := h.must("", "network", "ls", "dev"); !strings.Contains(out, "pythonhosted.org") || !strings.Contains(out, "Not locked") {
		t.Errorf("ls:\n%s", out)
	}

	h.must("", "network", "rm", "dev", "python")
	if strings.Contains(conf(), "pypi.org") || !strings.Contains(conf(), "example.com") {
		t.Errorf("rm python:\n%s", conf())
	}
	if _, err := h.run("", "network", "rm", "dev", "github"); err == nil || !strings.Contains(err.Error(), "none of those") {
		t.Errorf("rm of something not there: %v", err)
	}
	if _, err := h.run("", "network", "rm", "dev", "example.com"); err == nil || !strings.Contains(err.Error(), "limabean network open dev") {
		t.Errorf("rm of the last domain should point at open: %v", err)
	}

	h.must("", "network", "lock", "dev")
	if out := h.must("", "network", "ls", "dev"); !strings.Contains(out, "Locked: its user has no sudo or Docker") {
		t.Errorf("ls locked:\n%s", out)
	}
	h.must("", "network", "open", "dev")
	if strings.Contains(conf(), "allow") || !strings.Contains(conf(), "locked") {
		t.Errorf("open keeps the lock and drops the allowlist:\n%s", conf())
	}
	h.must("", "network", "unlock", "dev")
	if strings.Contains(conf(), "locked") {
		t.Errorf("unlock:\n%s", conf())
	}

	// A stopped VM gets the policy when it starts.
	h.world.vms["dev"] = "Stopped"
	h.world.calls = nil
	if out := h.must("", "network", "allow", "dev", "github"); !strings.Contains(out, "applies when it starts") || h.world.called("ssh") {
		t.Errorf("stopped VM:\n%s", out)
	}

	// Old VMs, failures and bad input.
	h.world.oldVMs["old"], h.world.vms["old"] = true, "Running"
	if out := h.must("", "network", "ls", "old"); !strings.Contains(out, "rebuild it") {
		t.Errorf("ls of an old VM:\n%s", out)
	}
	if _, err := h.run("", "network", "allow", "old", "github"); err == nil || !strings.Contains(err.Error(), "rebuild it first") {
		t.Errorf("allow on an old VM: %v", err)
	}
	h.world.vms["dev"] = "Running"
	h.world.failNetwork = true
	if out, err := h.run("", "network", "lock", "dev"); err == nil || !strings.Contains(out, "journalctl -u limabean-network") {
		t.Errorf("a failed apply: %v\n%s", err, out)
	}
	h.world.failNetwork = false
	for _, args := range [][]string{
		{"network", "allow", "dev", "bad domain"},
		{"network", "rm", "dev", "bad domain"},
		{"network", "allow", "nope", "github"},
		{"network", "ls", "nope"},
	} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("limabean %v should fail", args)
		}
	}
	if err := os.WriteFile(filepath.Join(h.dir, "network", "dev", "network.conf"), []byte("junk line\nallow\nlocked extra\nallow a.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := h.must("", "network", "ls", "dev"); !strings.Contains(out, "1 domain(s)") || strings.Contains(out, "Locked:") {
		t.Errorf("unknown lines are ignored:\n%s", out)
	}
}
