package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/claude"
	"github.com/rossedman/limabean/internal/github"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
	"github.com/rossedman/limabean/internal/ui"
)

// fakeWorld stands in for limactl, ssh and the Keychain.
type fakeWorld struct {
	vms      map[string]string // name → status
	calls    []string
	sshStdin map[string]string // VM → last stdin written over ssh
	keychain map[string]string
	unsaved  string            // what the unsaved-work check prints
	ghq      string            // what ghq list -p prints in every VM
	ss       string            // what ss -ltnpH prints in every VM
	latest   map[string]string // mise latest <spec> answers, on the Mac or in a VM
	activity map[string]string // VM → what stop --idle's check prints

	agentLoaded bool         // the autostop agent is loaded in launchd
	macPorts    map[int]bool // ports that answer on the Mac's localhost

	failStart bool // limactl start fails, with output
	failStop  bool // limactl stop fails

	limaHome       string          // where VM and disk directories live
	disks          map[string]bool // Lima disks
	failDiskList   bool
	failDiskCreate bool
	failDiskDelete bool
	failClone      bool // snapshot copies fail
	reboot         bool // the first boot left /var/run/reboot-required
	sshFail        bool // every ssh call fails
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{vms: map[string]string{}, sshStdin: map[string]string{}, keychain: map[string]string{}, activity: map[string]string{}, disks: map[string]bool{}, macPorts: map[int]bool{}, latest: map[string]string{}}
}

func (w *fakeWorld) Output(ctx context.Context, stdin io.Reader, _ bool, name string, args ...string) ([]byte, error) {
	w.calls = append(w.calls, name+" "+strings.Join(args, " "))
	in := ""
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		in = string(b)
	}
	switch name {
	case "limactl":
		switch args[0] {
		case "--version":
			return []byte("limactl version 2.2.1\n"), nil
		case "list":
			var b strings.Builder
			names := make([]string, 0, len(w.vms))
			for n := range w.vms {
				names = append(names, n)
			}
			sort.Strings(names) // limactl lists by name
			for _, n := range names {
				status := w.vms[n]
				fmt.Fprintf(&b, `{"name":%q,"status":%q,"dir":%q,"cpus":4,"memory":8589934592,"sshConfigFile":"/lima/%s/ssh.config","config":{"provision":[{"path":"/etc/limabean/mise/00-base.toml"}]}}`+"\n", n, status, filepath.Join(w.limaHome, n), n)
			}
			return []byte(b.String()), nil
		case "start":
			if w.failStart {
				return []byte("level=fatal msg=\"boom from lima\""), errors.New("exit status 1")
			}
			for _, a := range args {
				if name, ok := strings.CutPrefix(a, "--name="); ok {
					w.vms[name] = "Running"
					w.writeFile(filepath.Join(name, "disk"), "system of "+name)
					return nil, nil
				}
			}
			w.vms[args[len(args)-1]] = "Running"
		case "stop":
			if w.failStop {
				return []byte("stop failed"), errors.New("exit status 1")
			}
			w.vms[args[1]] = "Stopped"
		case "delete":
			delete(w.vms, args[len(args)-1])
		case "disk":
			switch args[1] {
			case "list":
				if w.failDiskList {
					return nil, errors.New("exit status 1")
				}
				var b strings.Builder
				for d := range w.disks {
					fmt.Fprintf(&b, `{"name":%q,"format":"raw"}`+"\n", d)
				}
				return []byte(b.String()), nil
			case "create":
				if w.failDiskCreate {
					return []byte("disk create failed"), errors.New("exit status 1")
				}
				w.disks[args[2]] = true
				w.writeFile(filepath.Join("_disks", args[2], "datadisk"), "empty code disk")
			case "delete":
				if w.failDiskDelete {
					return []byte("disk is in use"), errors.New("exit status 1")
				}
				delete(w.disks, args[2])
			}
		}
		return nil, nil
	case "ssh":
		if w.sshFail {
			return nil, errors.New("ssh: connection refused")
		}
		host := args[len(args)-2]
		vm := strings.TrimPrefix(host, "lima-")
		remote := args[len(args)-1]
		switch {
		case strings.Contains(remote, "reboot-required"):
			if w.reboot {
				return nil, nil
			}
			return nil, errors.New("exit 1") // nothing to restart for
		case remote == "bash -s": // the unsaved-work check
			return []byte(w.unsaved), nil
		case strings.HasPrefix(remote, "echo "):
			return []byte(strings.TrimPrefix(remote, "echo ") + "\n"), nil
		case remote == "false":
			err := exec.Command("sh", "-c", "exit 3").Run()
			return []byte("it failed\n"), err
		case remote == "sleep":
			<-ctx.Done()
			return []byte("partial"), ctx.Err()
		case remote == "sh -s": // stop --idle's check
			return []byte(w.activity[vm]), nil
		case strings.HasPrefix(remote, "mise latest "):
			spec := strings.Trim(strings.TrimPrefix(remote, "mise latest "), "'")
			if v, ok := w.latest[spec]; ok {
				return []byte(v + "\n"), nil
			}
			return nil, errors.New("exit status 1")
		case remote == "ss -ltnpH":
			return []byte(w.ss), nil
		case strings.HasPrefix(remote, "ghq list -p "):
			return []byte(w.ghq), nil
		case strings.HasSuffix(remote, "&& pwd"): // code's code directory
			return []byte("/home/me.linux/code\n"), nil
		case strings.Contains(remote, "grep -q"):
			if strings.Contains(w.sshStdin[vm], "export "+strings.Split(strings.Split(remote, "'^export ")[1], "=")[0]+"=") {
				return nil, nil
			}
			return nil, errors.New("exit 1")
		}
		w.sshStdin[vm] = in
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected program %s", name)
}

func (w *fakeWorld) Get(s string) (string, error) {
	if v, ok := w.keychain[s]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}
func (w *fakeWorld) Set(s, v string) error { w.keychain[s] = v; return nil }
func (w *fakeWorld) Delete(s string) error {
	if _, ok := w.keychain[s]; !ok {
		return errors.New("not found")
	}
	delete(w.keychain, s)
	return nil
}

type harness struct {
	t      *testing.T
	world  *fakeWorld
	dir    string // config dir
	claude string // settings.json
	github *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	h := &harness{t: t, world: newFakeWorld(), dir: filepath.Join(dir, "config"), claude: filepath.Join(dir, "claude", "settings.json")}
	h.world.limaHome = filepath.Join(dir, "lima")
	h.github = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer github_pat_good" {
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(h.github.Close)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("LIMABEAN_AWS_DIR", "") // never mount the real ~/.aws
	// A home of its own (doctor and init read ~/.ssh/config), and a fake
	// ssh-add whose answer FAKE_SSH_ADD controls: 0 keys loaded, 1 none.
	home := filepath.Join(dir, "home")
	bin := filepath.Join(dir, "bin")
	for _, d := range []string{filepath.Join(home, ".ssh"), bin} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "ssh-add"), []byte("#!/bin/sh\ncase ${FAKE_SSH_ADD:-0} in 0) echo '256 SHA256:x key (ED25519)';; *) exit $FAKE_SSH_ADD;; esac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return h
}

// run executes limabean with args and stdin, returning its output.
func (h *harness) run(stdin string, args ...string) (string, error) {
	h.t.Helper()
	var out bytes.Buffer
	return h.runUI(ui.NewPlain(&out), &out, stdin, args...)
}

// runTTY executes limabean as if in a terminal, typing keys into its one form.
func (h *harness) runTTY(keys []string, args ...string) (string, error) {
	h.t.Helper()
	return h.runForms([][]string{keys}, args...)
}

// runForms is runTTY for commands that show several forms: one key list each.
func (h *harness) runForms(forms [][]string, args ...string) (string, error) {
	h.t.Helper()
	var out bytes.Buffer
	readers := make([]io.Reader, len(forms))
	for i, keys := range forms {
		readers[i] = ui.Keys(keys...)
	}
	return h.runUI(ui.NewTerminalForms(&out, readers...), &out, "", args...)
}

func (h *harness) runUI(u *ui.UI, out *bytes.Buffer, stdin string, args ...string) (string, error) {
	h.t.Helper()
	a := h.app(u)
	root := NewRootCmd(a)
	root.SetArgs(append([]string{"--config", filepath.Join(h.dir, "config")}, args...))
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(out)
	root.SetErr(out)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// app returns an App wired to the fake world.
func (h *harness) app(u *ui.UI) *App {
	a := NewApp("test", os.DirFS(filepath.Join("..", "..")))
	a.UI = u
	a.execSSH = func(inst lima.Instance, command ...string) error {
		h.world.calls = append(h.world.calls, "exec ssh "+inst.Name+" "+strings.Join(command, " "))
		return nil
	}
	a.attachSSH = func(inst lima.Instance, command ...string) error {
		h.world.calls = append(h.world.calls, "attach ssh "+inst.Name+" "+strings.Join(command, " "))
		return nil
	}
	a.launchctl = func(_ context.Context, args ...string) ([]byte, error) {
		h.world.calls = append(h.world.calls, "launchctl "+strings.Join(args, " "))
		switch args[0] {
		case "bootstrap":
			h.world.agentLoaded = true
		case "bootout", "print":
			if !h.world.agentLoaded {
				return []byte("Could not find service"), errors.New("exit status 113")
			}
			h.world.agentLoaded = args[0] == "print"
		}
		return nil, nil
	}
	a.miseLatest = func(_ context.Context, spec string) (string, error) {
		h.world.calls = append(h.world.calls, "mise latest "+spec)
		if v, ok := h.world.latest[spec]; ok {
			return v, nil
		}
		return "", errors.New("mise ERROR no such tool\nmore detail")
	}
	a.cloneFile = func(_ context.Context, src, dst string) error {
		h.world.calls = append(h.world.calls, "clone "+filepath.Base(src))
		if h.world.failClone {
			return errors.New("not APFS")
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	}
	a.openEditor = func(_ context.Context, ide string, args ...string) error {
		h.world.calls = append(h.world.calls, "editor "+ide+" "+strings.Join(args, " "))
		return nil
	}
	a.macListens = func(port int) bool { return h.world.macPorts[port] }
	a.openURL = func(_ context.Context, url string) error {
		h.world.calls = append(h.world.calls, "open "+url)
		return nil
	}
	a.Lima = &lima.Client{Runner: h.world}
	a.Keychain = h.world
	a.Claude = claude.Settings{Path: h.claude}
	a.GitHub = github.Client{BaseURL: h.github.URL}
	a.keychainPrefix = "limabean"
	// A stand-in 1Password CLI: op read prints "op:<reference>".
	a.Resolver = secrets.Resolver{
		Keychain:   h.world.Get,
		LookupEnv:  os.LookupEnv,
		HasProgram: func(string) bool { return true },
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "op" {
				return []byte("op:" + args[len(args)-1]), nil
			}
			return secrets.RunCommand(ctx, name, args...)
		},
	}
	return a
}

// writeFile writes a file under the fake Lima home.
func (w *fakeWorld) writeFile(rel, body string) {
	p := filepath.Join(w.limaHome, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(body), 0o644)
}

// readFile reads a file under the fake Lima home.
func (w *fakeWorld) readFile(rel string) string {
	b, _ := os.ReadFile(filepath.Join(w.limaHome, rel))
	return string(b)
}

// called reports whether any recorded call starts with prefix.
func (w *fakeWorld) called(prefix string) bool {
	for _, c := range w.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func (h *harness) must(stdin string, args ...string) string {
	h.t.Helper()
	out, err := h.run(stdin, args...)
	if err != nil {
		h.t.Fatalf("limabean %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (h *harness) read(name string) string {
	b, _ := os.ReadFile(filepath.Join(h.dir, name))
	return string(b)
}

func TestNewCreatesInstallsSecretsAndRegisters(t *testing.T) {
	h := newHarness(t)
	t.Setenv("FROM_ENV", "env-value")
	h.must("", "secrets", "set", "SHARED", "env:FROM_ENV")
	out := h.must("", "new", "dev", "--secret", "OWN=plain-setting", "--cpus", "6")

	start := ""
	for _, c := range h.world.calls {
		if strings.HasPrefix(c, "limactl start --name=dev") {
			start = c
		}
	}
	for _, want := range []string{"--param codeDir=~/code", "--param gitConfigDir=~/.config/git", "--cpus 6", "/scratch.yaml"} {
		if !strings.Contains(start, want) {
			t.Errorf("limactl start missing %q: %s", want, start)
		}
	}
	if got := h.read("secrets.dev.env"); !strings.Contains(got, "OWN=plain-setting") {
		t.Errorf("VM secrets file = %q", got)
	}
	installed := h.world.sshStdin["dev"]
	if !strings.Contains(installed, "export SHARED='env-value'") || !strings.Contains(installed, "export OWN='plain-setting'") {
		t.Errorf("installed secrets = %q", installed)
	}
	names, _ := claude.Settings{Path: h.claude}.VMs()
	if strings.Join(names, ",") != "dev" {
		t.Errorf("Claude entries = %v", names)
	}
	for _, want := range []string{"Created dev", "Installed 2 secret(s)", "Added to Claude Desktop", "dev is ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if _, err := h.run("", "new", "dev"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second new dev: err = %v", err)
	}
	if _, err := h.run("", "new"); err == nil || !strings.Contains(err.Error(), "name the VM") {
		t.Errorf("new without a name in plain mode: err = %v", err)
	}
	if _, err := h.run("", "new", "bad_name"); err == nil {
		t.Errorf("new bad_name: want an error")
	}
}

func TestSecretsSetLsRm(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"

	// A pasted value goes to the Keychain and is referenced from there.
	h.must("s3cret value\n", "secrets", "set", "API_KEY", "--from-stdin")
	if h.world.keychain["limabean-secret-API_KEY"] != "s3cret value" {
		t.Fatalf("keychain = %v", h.world.keychain)
	}
	if !strings.Contains(h.read("secrets.env"), "API_KEY=keychain:limabean-secret-API_KEY") {
		t.Fatalf("shared file = %q", h.read("secrets.env"))
	}
	if !strings.Contains(h.world.sshStdin["dev"], "export API_KEY='s3cret value'") {
		t.Fatalf("set didn't sync to the running VM: %q", h.world.sshStdin["dev"])
	}

	// A reference for one VM.
	h.must("", "secrets", "set", "NPM_TOKEN", "op://Work/npm/token", "--vm", "dev", "--no-sync")
	if !strings.Contains(h.read("secrets.dev.env"), "NPM_TOKEN=op://Work/npm/token") {
		t.Fatalf("VM file = %q", h.read("secrets.dev.env"))
	}

	ls := h.must("", "secrets", "ls")
	for _, want := range []string{"API_KEY", "Keychain limabean-secret-API_KEY", "every VM", "NPM_TOKEN", "1Password Work/npm/token", "dev only"} {
		if !strings.Contains(ls, want) {
			t.Errorf("secrets ls missing %q:\n%s", want, ls)
		}
	}
	if strings.Contains(ls, "s3cret") {
		t.Errorf("secrets ls leaked a value:\n%s", ls)
	}

	h.must("", "secrets", "rm", "API_KEY")
	if _, ok := h.world.keychain["limabean-secret-API_KEY"]; ok {
		t.Errorf("owned Keychain item not deleted")
	}
	if strings.Contains(h.world.sshStdin["dev"], "API_KEY") {
		t.Errorf("rm didn't remove it from the VM: %q", h.world.sshStdin["dev"])
	}
	if _, err := h.run("", "secrets", "rm", "NOPE"); err == nil {
		t.Errorf("rm of a missing secret should fail")
	}
}

func TestUnresolvableSecretIsReportedAndOthersInstalled(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	h.must("", "secrets", "set", "GOOD", "fine", "--no-sync")
	h.must("", "secrets", "set", "MISSING", "keychain:no-such-item", "--no-sync")
	out, err := h.run("", "secrets", "sync")
	if err == nil {
		t.Fatalf("sync should fail when a secret can't be resolved:\n%s", out)
	}
	if !strings.Contains(out, "MISSING") || !strings.Contains(h.world.sshStdin["dev"], "export GOOD='fine'") {
		t.Fatalf("out=%s installed=%q", out, h.world.sshStdin["dev"])
	}
}

func TestGitHubLoginStatusLogout(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	if _, err := h.run("not-a-token\n", "github", "login", "--with-token"); err == nil {
		t.Fatal("a rejected token should fail")
	}
	out := h.must("github_pat_good\n", "github", "login", "--with-token")
	if !strings.Contains(out, "octocat") || h.world.keychain["limabean-github-token"] != "github_pat_good" {
		t.Fatalf("login: %s / %v", out, h.world.keychain)
	}
	if !strings.Contains(h.read("secrets.env"), "GITHUB_TOKEN=keychain:limabean-github-token") {
		t.Fatalf("shared file = %q", h.read("secrets.env"))
	}
	if !strings.Contains(h.world.sshStdin["dev"], "export GITHUB_TOKEN='github_pat_good'") {
		t.Fatalf("not installed in the VM: %q", h.world.sshStdin["dev"])
	}
	if status := h.must("", "github", "status"); !strings.Contains(status, "dev") || !strings.Contains(status, "installed") {
		t.Fatalf("status:\n%s", status)
	}
	h.must("", "github", "logout")
	if _, ok := h.world.keychain["limabean-github-token"]; ok || strings.Contains(h.read("secrets.env"), "GITHUB_TOKEN") {
		t.Fatalf("logout left the token behind")
	}
	if strings.Contains(h.world.sshStdin["dev"], "GITHUB_TOKEN") {
		t.Fatalf("logout didn't remove it from the VM")
	}
}

func TestRmChecksUnsavedWorkAndCleansUp(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "dev", "--secret", "OWN=x")
	h.world.unsaved = "~/code/github.com/me/app: unpushed commits\n"
	out, err := h.run("", "rm", "dev")
	if err == nil || !strings.Contains(out, "unpushed commits") {
		t.Fatalf("rm with unsaved work: err=%v\n%s", err, out)
	}
	if _, ok := h.world.vms["dev"]; !ok {
		t.Fatal("VM deleted despite unsaved work")
	}
	h.must("", "rm", "dev", "--force")
	if _, ok := h.world.vms["dev"]; ok {
		t.Fatal("VM not deleted")
	}
	if _, err := os.Stat(secrets.VMFile(h.dir, "dev")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("VM secrets file kept: %v", err)
	}
	if names, _ := (claude.Settings{Path: h.claude}).VMs(); len(names) != 0 {
		t.Errorf("Claude entry kept: %v", names)
	}
	if _, err := h.run("", "rm", "dev"); err == nil {
		t.Errorf("rm of a VM that's gone with nothing left should fail")
	}
}

func TestRmCleansUpAfterLimactlDelete(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "dev")
	delete(h.world.vms, "dev") // as if `limactl delete dev`
	out := h.must("", "rm", "dev")
	if !strings.Contains(out, "removed what was left") {
		t.Fatalf("out:\n%s", out)
	}
	if names, _ := (claude.Settings{Path: h.claude}).VMs(); len(names) != 0 {
		t.Errorf("Claude entry kept: %v", names)
	}
}

func TestLsAndPrune(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "dev")
	h.world.vms["other"] = "Stopped"
	// A Claude entry for a VM deleted behind limabean's back.
	if err := (claude.Settings{Path: h.claude}).Add("ghost"); err != nil {
		t.Fatal(err)
	}
	out := h.must("", "ls")
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "lima-dev") || !strings.Contains(out, "other") {
		t.Fatalf("ls:\n%s", out)
	}
	if !strings.Contains(out, "Removed ghost from Claude Desktop") {
		t.Errorf("ls didn't prune the ghost entry:\n%s", out)
	}
}

func TestLsJSON(t *testing.T) {
	h := newHarness(t)
	var stdout, stderr bytes.Buffer
	ls := func() []map[string]any {
		t.Helper()
		stdout.Reset()
		u := ui.NewPlain(&stdout)
		u.Err = &stderr
		if _, err := h.runUI(u, &stdout, "", "ls", "--json"); err != nil {
			t.Fatal(err)
		}
		var got []map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("stdout isn't JSON: %v\n%s", err, stdout.String())
		}
		return got
	}
	if got := ls(); got == nil || len(got) != 0 {
		t.Fatalf("no VMs should print [], got %v", got)
	}

	h.must("", "new", "dev")
	h.must("", "secrets", "set", "OWN", "plain", "--vm", "dev", "--no-sync")
	h.world.vms["other"] = "Stopped"
	if err := (claude.Settings{Path: h.claude}).Add("ghost"); err != nil {
		t.Fatal(err)
	}
	got := ls()
	if len(got) != 2 {
		t.Fatalf("want 2 VMs, got %v", got)
	}
	want := map[string]any{"name": "dev", "status": "Running", "env": "scratch", "cpus": 4.0, "memoryGiB": 8.0, "secrets": 1.0, "sshHost": "lima-dev"}
	for k, v := range want {
		if got[0][k] != v {
			t.Errorf("dev %s = %v, want %v", k, got[0][k], v)
		}
	}
	if got[1]["name"] != "other" || got[1]["env"] != "" {
		t.Errorf("other: %v", got[1])
	}
	if !strings.Contains(stderr.String(), "Removed ghost from Claude Desktop") {
		t.Errorf("tidy-up notes should go to stderr, got %q", stderr.String())
	}
}

func TestDoctorPlain(t *testing.T) {
	h := newHarness(t)
	h.must("", "secrets", "set", "BROKEN", "keychain:missing", "--no-sync")
	out, err := h.run("", "doctor")
	if err == nil {
		t.Fatalf("doctor should fail with a missing Keychain item:\n%s", out)
	}
	for _, want := range []string{"Lima 2.2.1", "BROKEN (secrets.env): no Keychain item missing"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor missing %q:\n%s", want, out)
		}
	}
}

func TestInitPlainSavesSuggestions(t *testing.T) {
	h := newHarness(t)
	h.must("", "init")
	cfg := h.read("config")
	if !strings.Contains(cfg, `CODE_DIR="~/code"`) || !strings.Contains(cfg, "GIT_CONFIG_DIR=") {
		t.Fatalf("config = %q", cfg)
	}
}

func TestVersionAndCompletion(t *testing.T) {
	h := newHarness(t)
	if out := h.must("", "version"); !strings.HasPrefix(out, "limabean test") {
		t.Errorf("version = %q", out)
	}
	if out := h.must("", "completion", "zsh"); !strings.Contains(out, "#compdef limabean") {
		t.Errorf("zsh completion doesn't start with #compdef")
	}
}

func TestClaudeSettingsJSONStaysValid(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "a")
	h.must("", "new", "b")
	data, _ := os.ReadFile(h.claude)
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("settings.json invalid: %v\n%s", err, data)
	}
}

func TestHelpIsPlainWhenNotATerminal(t *testing.T) {
	h := newHarness(t)
	out := h.must("", "--help")
	if strings.Contains(out, "\x1b[") {
		t.Errorf("help has color codes outside a terminal:\n%q", out)
	}
	for _, want := range []string{"Usage:", "Examples:", "Setup:", "VMs:", "Secrets and access:", "Flags:"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
	if sub := h.must("", "secrets", "set", "--help"); !strings.Contains(sub, "Global flags:") || !strings.Contains(sub, "--from-stdin") {
		t.Errorf("subcommand help:\n%s", sub)
	}
	// Workflow order, not alphabetical.
	if strings.Index(out, "  new ") > strings.Index(out, "  get ") {
		t.Errorf("commands aren't in workflow order:\n%s", out)
	}
}
