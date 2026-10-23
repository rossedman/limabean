package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func readInfo(t *testing.T, h *harness, vm string) vmInfo {
	t.Helper()
	raw := h.world.sshStdin[vm+".json"]
	var info vmInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		t.Fatalf("%s's vm.json isn't JSON: %v\n%s", vm, err, raw)
	}
	return info
}

func TestVMInfoOnNew(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "plain", "--with", "go")
	info := readInfo(t, h, "plain")
	if info.Name != "plain" || info.Env != "scratch" || strings.Join(info.Packs, ",") != "core,go" ||
		info.Project != nil || info.WrittenBy != "limabean test" || !strings.Contains(info.Ports, "localhost:<port>") ||
		info.Logs["provision"] == "" || info.Written == "" {
		t.Errorf("info = %+v", info)
	}
}

func TestVMInfoOnUp(t *testing.T) {
	h := newHarness(t)
	h.world.ghq = "/home/me/code/github.com/acme/web\n"
	h.must("", "secrets", "set", "API_KEY", "s3cret-value", "--no-sync")
	file := projectFile(t, "web", `repo: acme/web
ports: [3000]
vars:
  NODE_ENV: very-specific-value
secrets: [API_KEY]
setup: npm ci
`)
	h.must("", "up", "--file", file, "--yes")
	raw := h.world.sshStdin["web.json"]
	// Names, never values: neither the secret's value nor its reference.
	for _, leak := range []string{"s3cret-value", "API_KEY", "very-specific-value"} {
		if strings.Contains(raw, leak) {
			t.Errorf("vm.json has %q:\n%s", leak, raw)
		}
	}
	info := readInfo(t, h, "web")
	p := info.Project
	if p == nil || p.Repo != "acme/web" || p.Clone != "/home/me/code/github.com/acme/web" || len(p.Ports) != 1 || p.Ports[0] != 3000 ||
		strings.Join(p.Vars, ",") != "NODE_ENV" || p.Setup != "npm ci" || info.Logs["setup"] == "" || info.Logs["start"] != "" {
		t.Fatalf("project = %+v, logs = %v", p, info.Logs)
	}

	// Every up writes it again, with what the file says now.
	rewrite(t, file, "repo: acme/web\nports: [8080]\n")
	delete(h.world.sshStdin, "web.json")
	h.must("", "up", "--file", file, "--yes")
	if p := readInfo(t, h, "web").Project; p == nil || p.Ports[0] != 8080 || p.Clone == "" || p.Setup != "" {
		t.Errorf("after the second up: %+v", p)
	}
}

func TestVMInfoWriteFails(t *testing.T) {
	h := newHarness(t)
	h.world.infoErr = true
	out := h.must("", "new", "broken")
	if !strings.Contains(out, "Couldn't write broken's info file") {
		t.Errorf("no warning:\n%s", out)
	}
}

// limabean-info runs for real with sh and jq, on the file limabean writes.
func TestLimabeanInfoScript(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("needs jq")
	}
	h := newHarness(t)
	h.world.ghq = "/home/me/code/github.com/acme/web\n"
	file := projectFile(t, "web", "repo: acme/web\nports: [3000]\nsetup: |\n  npm ci\n  npm test\n")
	h.must("", "up", "--file", file, "--yes")

	home := t.TempDir()
	dir := filepath.Join(home, ".config", "limabean")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("vm.json", h.world.sshStdin["web.json"])
	write("secrets.env", "export GITHUB_TOKEN='ghp_secret'\nexport API_KEY='a b'\n")
	write("network.conf", "# policy\nlocked\nallow github.com\nallow registry.npmjs.org\n")
	run := func(args ...string) (string, error) {
		cmd := exec.Command("sh", append([]string{"../../templates/files/limabean-info"}, args...)...)
		cmd.Env = append(os.Environ(), "HOME="+home, "LIMABEAN_NETWORK_CONF="+filepath.Join(dir, "network.conf"))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	out, err := run()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{"web (scratch), a limabean VM", "packs:     core", "only github.com, registry.npmjs.org, locked",
		"secrets:   API_KEY, GITHUB_TOKEN", "clone:     /home/me/code/github.com/acme/web", "http://localhost:3000",
		"setup:     npm ci; npm test", "Written by limabean test"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ghp_secret") {
		t.Errorf("printed a secret's value:\n%s", out)
	}

	out, err = run("--json")
	var all struct {
		Name    string   `json:"name"`
		Secrets []string `json:"secrets"`
		Network struct {
			Allow  []string `json:"allow"`
			Locked bool     `json:"locked"`
		} `json:"network"`
	}
	if err != nil || json.Unmarshal([]byte(out), &all) != nil || all.Name != "web" || len(all.Secrets) != 2 ||
		!all.Network.Locked || len(all.Network.Allow) != 2 || strings.Contains(out, "ghp_secret") {
		t.Errorf("--json: %v\n%s", err, out)
	}

	// No vm.json yet, an open network, a bad option.
	_ = os.Remove(filepath.Join(dir, "vm.json"))
	write("network.conf", "")
	if out, err := run(); err != nil || !strings.Contains(out, "no vm.json yet") || !strings.Contains(out, "network:   open") {
		t.Errorf("without vm.json: %v\n%s", err, out)
	}
	if out, err := run("--nope"); err == nil || !strings.Contains(out, "unknown option") {
		t.Errorf("bad option: %v\n%s", err, out)
	}
}
