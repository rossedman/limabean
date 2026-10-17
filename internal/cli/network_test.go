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

func TestNetworkLearnAndLog(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.world.vms["open"] = "Running"
	if _, err := h.run("", "network", "log", "open"); err == nil || !strings.Contains(err.Error(), "isn't recorded") && !strings.Contains(err.Error(), "aren't recorded") {
		t.Errorf("log of an open VM: %v", err)
	}
	if _, err := h.run("", "network", "learn", "open", "--done"); err == nil || !strings.Contains(err.Error(), "isn't learning") {
		t.Errorf("--done without learning: %v", err)
	}

	out := h.must("", "new", "agent", "--learn", "--allow", "github")
	if !strings.Contains(out, "Network: learning") || !strings.Contains(h.read(filepath.Join("network", "agent", "network.conf")), "learn\n") {
		t.Fatalf("new --learn:\n%s", out)
	}
	if out := h.must("", "network", "ls", "agent"); !strings.Contains(out, "Learning") || !strings.Contains(out, "github.com") {
		t.Errorf("ls while learning:\n%s", out)
	}
	if _, err := h.run("", "network", "learn", "agent", "--done"); err != nil {
		t.Errorf("--done with only an allowlist so far should keep it: %v", err)
	}
	h.must("", "network", "learn", "agent")

	h.world.lookups = "      4 registry.npmjs.org\n      2 api.github.com\n      1 files.pythonhosted.org.\n      1 ntp.ubuntu.com\n      3 www.example.co.uk\n      1 localhost\n garbage\n"
	out = h.must("", "network", "log", "agent")
	for _, want := range []string{"registry.npmjs.org", "4", "seen", "learn agent --done", "example.co.uk", "npmjs.org", "pythonhosted.org"} {
		if !strings.Contains(out, want) {
			t.Errorf("log while learning missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "localhost") {
		t.Errorf("names without a dot are left out:\n%s", out)
	}

	out = h.must("", "network", "learn", "agent", "--done")
	if !strings.Contains(out, "Allowed: example.co.uk, npmjs.org, pythonhosted.org") {
		t.Errorf("--done:\n%s", out)
	}
	conf := h.read(filepath.Join("network", "agent", "network.conf"))
	if strings.Contains(conf, "learn") || !strings.Contains(conf, "allow npmjs.org\n") || !strings.Contains(conf, "allow github.com\n") || strings.Contains(conf, "api.github.com") {
		t.Errorf("network.conf after --done:\n%s", conf)
	}

	h.world.lookups = "      1 api.github.com\n      2 evil.example.com\n"
	out = h.must("", "network", "log", "agent", "--blocked")
	if strings.Contains(out, "api.github.com") || !strings.Contains(out, "evil.example.com") || !strings.Contains(out, "blocked") || !strings.Contains(out, "network allow agent example.com") {
		t.Errorf("log --blocked:\n%s", out)
	}
	var entries []lookupEntry
	if err := json.Unmarshal([]byte(h.must("", "network", "log", "agent", "--json")), &entries); err != nil || len(entries) != 2 || !entries[0].Allowed || entries[1].Allowed {
		t.Errorf("log --json: %v %+v", err, entries)
	}
	h.world.lookups = ""
	if out := h.must("", "network", "log", "agent"); !strings.Contains(out, "No lookups") {
		t.Errorf("empty log:\n%s", out)
	}
	h.must("", "network", "learn", "agent")
	if _, err := h.run("", "network", "learn", "agent", "--done"); err != nil {
		t.Errorf("--done keeping the allowlist: %v", err)
	}
	h.must("", "network", "open", "agent")
	h.must("", "network", "learn", "agent")
	if _, err := h.run("", "network", "learn", "agent", "--done"); err == nil || !strings.Contains(err.Error(), "nothing was looked up") {
		t.Errorf("--done with nothing: %v", err)
	}
	h.world.sshFail = true
	if _, err := h.run("", "network", "log", "agent"); err == nil {
		t.Error("an unreachable VM should fail")
	}
	h.world.sshFail = false
	h.world.vms["agent"] = "Stopped"
	for _, args := range [][]string{{"network", "log", "agent"}, {"network", "learn", "agent", "--done"}} {
		if _, err := h.run("", args...); err == nil || !strings.Contains(err.Error(), "stopped") {
			t.Errorf("%v on a stopped VM: %v", args, err)
		}
	}
	if out := h.must("", "network", "open", "agent"); !strings.Contains(out, "open") || strings.Contains(h.read(filepath.Join("network", "agent", "network.conf")), "learn") {
		t.Errorf("open should end learning:\n%s", out)
	}
}

func TestBaseDomain(t *testing.T) {
	for in, want := range map[string]string{"a.b.github.com": "github.com", "github.com": "github.com", "x.example.co.uk": "example.co.uk", "example.co.uk": "example.co.uk", "deep.cdn.example.com.au": "example.com.au", "a.io": "a.io", "x.y.io": "y.io"} {
		if got := baseDomain(in); got != want {
			t.Errorf("baseDomain(%q) = %q, want %q", in, got, want)
		}
	}
}
