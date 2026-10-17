package cli

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rossedman/limabean/internal/claude"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/ui"
)

func (h *harness) home() string { return os.Getenv("HOME") }

func (h *harness) writeSSHConfig(body string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.home(), ".ssh", "config"), []byte(body), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

// --- VMs ---------------------------------------------------------------

func TestStop(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	if out := h.must("", "stop", "dev"); !strings.Contains(out, "Stopped dev") || h.world.vms["dev"] != "Stopped" {
		t.Fatalf("stop: %s / %v", out, h.world.vms)
	}
	if out := h.must("", "stop", "dev"); !strings.Contains(out, "already stopped") {
		t.Fatalf("second stop: %s", out)
	}
	if _, err := h.run("", "stop", "nope"); err == nil {
		t.Error("stopping a missing VM should fail")
	}
	if _, err := h.run("", "stop"); err == nil || !strings.Contains(err.Error(), "name a VM") {
		t.Errorf("stop without a name in plain mode: %v", err)
	}
}

func TestStopIdle(t *testing.T) {
	h := newHarness(t)
	for _, n := range []string{"busy", "fresh", "idle", "off", "quiet"} {
		h.world.vms[n] = "Running"
	}
	h.world.vms["off"] = "Stopped"
	const now = 1_800_000_000
	at := func(ago time.Duration, busy string) string {
		return fmt.Sprintf("%d %d %s\n", now, now-int64(ago.Seconds()), busy)
	}
	h.world.activity["busy"] = at(5*time.Hour, "an editor is connected")
	h.world.activity["fresh"] = at(20*time.Minute, "")
	h.world.activity["idle"] = at(3*time.Hour+7*time.Minute, "")
	h.world.activity["quiet"] = "garbage"
	if err := (claude.Settings{Path: h.claude}).Add("idle"); err != nil {
		t.Fatal(err)
	}

	out, err := h.run("", "stop", "--idle", "2h", "--dry-run")
	if !errors.Is(err, errSilent) {
		t.Fatalf("a VM that couldn't be checked should fail the run: %v", err)
	}
	for _, want := range []string{
		"busy is in use: an editor is connected",
		"fresh was used 20m ago",
		"idle has been idle for 3h07m, so it would stop",
		"quiet: couldn't tell whether it's in use",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run missing %q:\n%s", want, out)
		}
	}
	if h.world.called("limactl stop") {
		t.Fatalf("--dry-run stopped something: %v", h.world.calls)
	}

	h.world.activity["quiet"] = at(10*time.Hour, "")
	out = h.must("", "stop", "--idle", "2h")
	if h.world.vms["idle"] != "Stopped" || h.world.vms["quiet"] != "Stopped" || h.world.vms["busy"] != "Running" || h.world.vms["fresh"] != "Running" {
		t.Fatalf("stopped the wrong VMs: %v\n%s", h.world.vms, out)
	}
	if !strings.Contains(out, "Stopped idle") {
		t.Errorf("out:\n%s", out)
	}
	if names, _ := (claude.Settings{Path: h.claude}).VMs(); len(names) != 0 {
		t.Errorf("a stopped VM kept its Claude entry: %v", names)
	}

	// One VM by name.
	if out := h.must("", "stop", "fresh", "--idle", "10m"); h.world.vms["fresh"] != "Stopped" {
		t.Fatalf("named VM: %v\n%s", h.world.vms, out)
	}
	if out := h.must("", "stop", "off", "--idle", "10m"); !strings.Contains(out, "off is already stopped") {
		t.Errorf("out:\n%s", out)
	}
	h.world.vms["busy"] = "Stopped"
	if out := h.must("", "stop", "--idle", "2h"); !strings.Contains(out, "No VMs are running") {
		t.Errorf("out:\n%s", out)
	}

	for _, args := range [][]string{{"stop", "--dry-run"}, {"stop", "--idle", "30s"}, {"stop", "nope", "--idle", "1h"}} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("limabean %v should fail", args)
		}
	}
	h.world.vms["busy"] = "Running"
	h.world.activity["busy"] = at(3*time.Hour, "")
	h.world.failStop = true
	if _, err := h.run("", "stop", "--idle", "1h"); err == nil {
		t.Error("a VM that won't stop should fail the run")
	}
}

func TestAutostop(t *testing.T) {
	h := newHarness(t)
	plist := filepath.Join(h.home(), "Library", "LaunchAgents", autostopLabel+".plist")
	if out := h.must("", "autostop", "status"); !strings.Contains(out, "Autostop is off") {
		t.Fatalf("status before on:\n%s", out)
	}
	if _, err := h.run("", "autostop", "on"); err == nil || !strings.Contains(err.Error(), "limactl isn't on your PATH") {
		t.Fatalf("on without limactl: %v", err)
	}
	bin := filepath.Join(h.home(), "lima", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "limactl"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := h.run("", "autostop", "on", "--after", "5m"); err == nil {
		t.Error("--after shorter than the check interval should fail")
	}
	out := h.must("", "autostop", "on", "--after", "90m")
	if !strings.Contains(out, "nobody has used for 1h30m are stopped (checked every 10m)") || !h.world.agentLoaded {
		t.Fatalf("on:\n%s", out)
	}
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<string>--idle</string>\n    <string>1h30m</string>", "<string>--plain</string>", "<string>--config</string>", "<string>" + bin + ":", "<integer>600</integer>", "limabean-autostop.log"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("plist missing %q:\n%s", want, body)
		}
	}
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict, dec.Entity = true, xml.HTMLEntity
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("the plist isn't well-formed XML: %v", err)
		}
	}

	// Again with a new limit: the loaded agent is replaced.
	h.world.calls = nil
	h.must("", "autostop", "on")
	if !h.world.called("launchctl bootout gui/") || !h.world.called("launchctl bootstrap gui/") || autostopAfter(mustRead(t, plist)) != "2h" {
		t.Fatalf("on again: %v", h.world.calls)
	}
	if out := h.must("", "autostop", "status"); !strings.Contains(out, "nobody has used for 2h are stopped") {
		t.Fatalf("status:\n%s", out)
	}
	h.world.agentLoaded = false
	if out, err := h.run("", "autostop", "status"); err == nil || !strings.Contains(out, "installed but not loaded") {
		t.Fatalf("status when unloaded: %v\n%s", err, out)
	}
	h.world.agentLoaded = true
	if out := h.must("", "autostop", "off"); !strings.Contains(out, "Autostop is off") || h.world.agentLoaded {
		t.Fatalf("off:\n%s", out)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Errorf("off left the plist: %v", err)
	}
	if out := h.must("", "autostop", "off"); !strings.Contains(out, "already off") {
		t.Errorf("second off:\n%s", out)
	}
}

func TestAutostopHelpers(t *testing.T) {
	for d, want := range map[time.Duration]string{2 * time.Hour: "2h", 90 * time.Minute: "1h30m", 45 * time.Minute: "45m", 30 * time.Second: "30s"} {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%v) = %s, want %s", d, got, want)
		}
	}
	for d, want := range map[time.Duration]string{30 * time.Second: "under a minute", 45 * time.Minute: "45m", 125 * time.Minute: "2h05m"} {
		if got := roughDuration(d); got != want {
			t.Errorf("roughDuration(%v) = %s, want %s", d, got, want)
		}
	}
	if !strings.Contains(string(renderPlist([]string{"/a&b/limabean"}, "/x", time.Minute)), "<string>/a&amp;b/limabean</string>") {
		t.Error("plist values aren't escaped")
	}
	if autostopAfter([]byte("<plist/>")) != "" {
		t.Error("a plist without --idle has no limit")
	}
}

func TestAutostopFindsItself(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := &App{}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if got := a.selfPath(); got != exe {
		t.Errorf("without limabean on PATH: %s, want %s", got, exe)
	}
	if err := os.WriteFile(filepath.Join(bin, "limabean"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := a.selfPath(); got != exe {
		t.Errorf("another limabean on PATH: %s, want %s", got, exe)
	}
	link := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(link, "limabean")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", link)
	if got := a.selfPath(); got != filepath.Join(link, "limabean") {
		t.Errorf("a link to this binary on PATH: %s", got)
	}
	// A service that doesn't exist: an error from launchctl, or no launchctl.
	if _, err := runLaunchctl(context.Background(), "print", "gui/0/"+autostopLabel+".test-missing"); err == nil {
		t.Error("printing a missing launchd service should fail")
	}
}

func TestIdleStateBadOutput(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	for _, bad := range []string{"", "12", "now later"} {
		h.world.activity["dev"] = bad
		if out, err := h.run("", "stop", "--idle", "1h"); err == nil || !strings.Contains(out, "unexpected output") {
			t.Errorf("activity %q: %v\n%s", bad, err, out)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPickerAndSingleVM(t *testing.T) {
	h := newHarness(t)
	if _, err := h.run("", "stop"); err == nil || !strings.Contains(err.Error(), "no VMs yet") {
		t.Errorf("no VMs: %v", err)
	}
	h.world.vms["only"] = "Running"
	if _, err := h.runTTY(nil, "stop"); err != nil || h.world.vms["only"] != "Stopped" {
		t.Fatalf("a single VM is picked without asking: %v %v", err, h.world.vms)
	}
	h.world.vms["alpha"], h.world.vms["beta"] = "Running", "Running"
	// alpha, beta, only: pick the second.
	if _, err := h.runTTY([]string{ui.Down, ui.Enter}, "stop"); err != nil {
		t.Fatal(err)
	}
	if h.world.vms["beta"] != "Stopped" && h.world.vms["alpha"] != "Stopped" {
		t.Fatalf("picker stopped nothing: %v", h.world.vms)
	}
}

func TestSSHAndGet(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Stopped"
	h.must("", "ssh", "dev", "ls", "-la")
	if !h.world.called("limactl start --tty=false dev") || !h.world.called("exec ssh dev ls -la") {
		t.Fatalf("ssh should start the VM, then exec: %v", h.world.calls)
	}
	h.must("", "get", "dev", "acme/repo", "-p")
	if !h.world.called("attach ssh dev ghq get -p acme/repo") {
		t.Fatalf("get should move flags first: %v", h.world.calls)
	}
	if _, err := h.run("", "get", "acme/other"); err == nil {
		t.Error("plain mode shouldn't guess the VM, even with one")
	}
	if _, err := h.runTTY(nil, "get", "acme/other"); err != nil { // the only VM is picked
		t.Fatal(err)
	}
	if !h.world.called("attach ssh dev ghq get acme/other") {
		t.Fatalf("get with one VM: %v", h.world.calls)
	}
	if _, err := h.run("", "get"); err == nil {
		t.Error("get without a repository should fail")
	}
	if _, err := h.run("", "ssh", "nope"); err == nil {
		t.Error("ssh to a missing VM should fail")
	}
	if _, err := h.run("", "get", "nope", "acme/x"); err == nil {
		t.Error("get into a missing VM should fail")
	}
}

func TestCode(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Stopped"
	h.world.ghq = "/home/me.linux/code/github.com/rossedman/limabean\n" +
		"/home/me.linux/code/github.com/rossedman/limabean-docs\n" +
		"/home/me.linux/code/github.com/acme/limabean\n"

	out := h.must("", "code", "dev")
	if !h.world.called("limactl start --tty=false dev") || !h.world.called("editor code --remote ssh-remote+lima-dev /home/me.linux/code") {
		t.Fatalf("code should start the VM, then open ~/code: %v", h.world.calls)
	}
	if !strings.Contains(out, "doesn't include Lima's host entries") {
		t.Errorf("no warning about the missing Include:\n%s", out)
	}

	// A full match wins over a longer name; an owner/repo picks between owners.
	h.writeSSHConfig("Include ~/.lima/*/ssh.config\n")
	out = h.must("", "code", "dev", "rossedman/limabean", "--ide", "cursor")
	if !h.world.called("editor cursor --remote ssh-remote+lima-dev /home/me.linux/code/github.com/rossedman/limabean") {
		t.Fatalf("calls: %v", h.world.calls)
	}
	if strings.Contains(out, "Include") {
		t.Errorf("warned with the Include in place:\n%s", out)
	}
	h.must("", "code", "dev", "https://github.com/acme/limabean.git")
	if !h.world.called("editor code --remote ssh-remote+lima-dev /home/me.linux/code/github.com/acme/limabean") {
		t.Fatalf("a URL should match its clone: %v", h.world.calls)
	}
	if !strings.Contains(strings.Join(h.world.calls, "\n"), "ghq list -p 'github.com/acme/limabean'") {
		t.Errorf("ghq should get the URL as host/owner/repo: %v", h.world.calls)
	}
	h.must("", "code", "dev", "/srv/app")
	if !h.world.called("editor code --remote ssh-remote+lima-dev /srv/app") {
		t.Fatalf("an absolute path is used as is: %v", h.world.calls)
	}

	// Two full matches: an error in plain mode, a picker in a terminal.
	h.world.calls = nil
	if _, err := h.run("", "code", "dev", "limabean"); err == nil || !strings.Contains(err.Error(), "matches several") {
		t.Errorf("ambiguous in plain mode: %v", err)
	}
	if _, err := h.runTTY([]string{ui.Down, ui.Enter}, "code", "dev", "limabean"); err != nil {
		t.Fatal(err)
	}
	if !h.world.called("editor code --remote ssh-remote+lima-dev /home/me.linux/code/github.com/acme/limabean") {
		t.Fatalf("picker: %v", h.world.calls)
	}

	// One argument that isn't a VM is a repository; with one VM, it's picked.
	h.world.calls = nil
	if _, err := h.runTTY(nil, "code", "rossedman/limabean"); err != nil {
		t.Fatal(err)
	}
	if !h.world.called("editor code --remote ssh-remote+lima-dev /home/me.linux/code/github.com/rossedman/limabean") {
		t.Fatalf("one argument as a repository: %v", h.world.calls)
	}

	h.world.ghq = ""
	if _, err := h.run("", "code", "dev", "nothing"); err == nil || !strings.Contains(err.Error(), "limabean get dev nothing") {
		t.Errorf("no match should suggest get: %v", err)
	}
	if _, err := h.run("", "code", "nope", "x"); err == nil {
		t.Error("code in a missing VM should fail")
	}
	h.world.sshFail = true
	if _, err := h.run("", "code", "dev"); err == nil {
		t.Error("code should fail when the VM can't be reached")
	}
	if _, err := h.run("", "code", "dev", "x"); err == nil {
		t.Error("code should fail when ghq can't be run")
	}
}

func TestCodeHelpers(t *testing.T) {
	for in, want := range map[string]string{"~": `"$HOME"`, "~/code": `"$HOME"/'code'`, "/abs dir": `'/abs dir'`} {
		if got := homeQuote(in); got != want {
			t.Errorf("homeQuote(%q) = %s, want %s", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"limabean":                               "limabean",
		"rossedman/limabean/":                    "rossedman/limabean",
		"https://github.com/acme/limabean.git":   "github.com/acme/limabean",
		"ssh://git@github.com/acme/limabean.git": "github.com/acme/limabean",
		"git@github.com:acme/limabean.git":       "github.com/acme/limabean",
	} {
		if got := repoQuery(in); got != want {
			t.Errorf("repoQuery(%q) = %q, want %q", in, got, want)
		}
	}
	t.Setenv("PATH", t.TempDir())
	for ide, want := range map[string]string{"code": "Install 'code'", "cursor": "Install 'cursor'", "zed": "zed isn't on your PATH"} {
		if err := openEditor(context.Background(), ide); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("openEditor(%s): %v", ide, err)
		}
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "code"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := openEditor(context.Background(), "code", "--remote", "ssh-remote+lima-dev", "/x"); err != nil {
		t.Errorf("openEditor with code on PATH: %v", err)
	}
}

func TestCodeDisk(t *testing.T) {
	h := newHarness(t)
	disk := codeDiskName("dev")
	if len("lima-"+disk) > 16 || codeDiskName("a-very-long-vm-name-indeed") == disk {
		t.Fatalf("code disk names must fit an ext4 label and differ per VM: %s", disk)
	}

	out := h.must("", "new", "dev")
	if !h.world.disks[disk] || !strings.Contains(out, "~/code is on its own disk") {
		t.Fatalf("new should create a code disk:\n%s", out)
	}
	if !h.world.called("limactl disk create "+disk+" --size 50GiB --format raw") ||
		!strings.Contains(strings.Join(h.world.calls, "\n"), `--param codeDisk=`+disk) ||
		!strings.Contains(strings.Join(h.world.calls, "\n"), `.additionalDisks += [{"name": "`+disk+`", "format": true, "fsType": "ext4"}]`) {
		t.Fatalf("calls: %v", h.world.calls)
	}

	// Rebuild keeps the disk: no unsaved-work check, a clean shutdown
	// before the delete, and the same disk attached again.
	h.world.unsaved = "~/code/github.com/me/app: unpushed commits\n"
	h.world.calls = nil
	out = h.must("", "rebuild", "dev")
	if !strings.Contains(out, "Repositories in ~/code were kept") || strings.Contains(out, "would be lost") {
		t.Fatalf("rebuild with a code disk:\n%s", out)
	}
	calls := strings.Join(h.world.calls, "\n")
	if stop, del := strings.Index(calls, "limactl stop dev"), strings.Index(calls, "limactl delete --force dev"); stop < 0 || stop > del {
		t.Fatalf("rebuild should stop the VM before deleting it: %v", h.world.calls)
	}
	if h.world.called("limactl disk create") || !strings.Contains(calls, "--param codeDisk="+disk) {
		t.Fatalf("rebuild should reuse the disk: %v", h.world.calls)
	}

	// A disk is reattached even with CODE_DISK turned off since.
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.world.calls = nil
	h.must("", "rebuild", "dev", "--force")
	if !strings.Contains(strings.Join(h.world.calls, "\n"), "--param codeDisk="+disk) {
		t.Fatalf("an existing disk should be kept: %v", h.world.calls)
	}

	// rm checks for unsaved work (the disk goes too), then deletes the disk.
	if _, err := h.run("", "rm", "dev"); err == nil {
		t.Fatal("rm should stop at unsaved work, code disk or not")
	}
	h.world.unsaved = ""
	if out := h.must("", "rm", "dev"); !strings.Contains(out, "Deleted dev's code disk") || h.world.disks[disk] {
		t.Fatalf("rm should delete the code disk:\n%s", out)
	}

	// Without CODE_DISK, new VMs have none and rebuild checks for unsaved work.
	h.must("", "new", "plain")
	if h.world.disks[codeDiskName("plain")] {
		t.Fatal("CODE_DISK empty still made a disk")
	}
	h.world.unsaved = "~/code/x: stashes\n"
	if out, err := h.run("", "rebuild", "plain"); err == nil || !strings.Contains(out, "would be lost") {
		t.Fatalf("rebuild without a code disk should check for unsaved work: %v\n%s", err, out)
	}

	// A disk left by a VM deleted with limactl is cleaned up by rm.
	h.world.disks[codeDiskName("ghost")] = true
	if out := h.must("", "rm", "ghost"); !strings.Contains(out, "Deleted ghost's code disk") {
		t.Fatalf("rm of a VM that's gone:\n%s", out)
	}
	h.world.disks[codeDiskName("stuck")] = true
	h.world.failDiskDelete = true
	if out := h.must("", "rm", "stuck"); !strings.Contains(out, "limactl disk delete "+codeDiskName("stuck")) {
		t.Fatalf("a disk that won't delete should say how:\n%s", out)
	}

	h.world.failDiskCreate = true
	t.Setenv("LIMABEAN_CODE_DISK", "20")
	if _, err := h.run("", "new", "broken"); err == nil {
		t.Error("new should fail when the code disk can't be created")
	}
	h.world.failDiskList = true
	if _, err := h.run("", "new", "broken2"); err == nil {
		t.Error("new should fail when Lima can't list disks")
	}
}

func TestStartFailureShowsLimaOutput(t *testing.T) {
	h := newHarness(t)
	h.world.failStart = true
	out, err := h.run("", "new", "dev")
	if err == nil || !strings.Contains(out, "boom from lima") {
		t.Fatalf("err=%v out=%s", err, out)
	}
	h.world.vms["stopped"] = "Stopped"
	if _, err := h.run("", "ssh", "stopped"); err == nil {
		t.Error("ssh should fail when the VM won't start")
	}
}

func TestNewRestartsForUpdatesAndMountsAWS(t *testing.T) {
	h := newHarness(t)
	h.world.reboot = true
	aws := filepath.Join(h.home(), ".aws")
	if err := os.MkdirAll(aws, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIMABEAN_AWS_DIR", aws)
	t.Setenv("LIMABEAN_CLAUDE_DESKTOP", "no")
	t.Setenv("LIMABEAN_CODE_DISK", "")
	out := h.must("", "new", "dev")
	for _, want := range []string{"Installed updates and restarted", "Mounted " + aws, "No secrets set up"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if !h.world.called("limactl stop dev") || !h.world.called(`limactl start --name=dev --tty=false --param codeDir=~/code --param gitConfigDir=~/.config/git --set .mounts`) {
		t.Errorf("calls: %v", h.world.calls)
	}
	if names, _ := (claude.Settings{Path: h.claude}).VMs(); len(names) != 0 {
		t.Errorf("CLAUDE_DESKTOP=no still registered: %v", names)
	}
	if _, err := h.run("", "new", "x", "--secret", "bad"); err == nil {
		t.Error("a malformed --secret should fail")
	}
	if _, err := h.run("", "new", "x", "nosuchenv"); err == nil || !strings.Contains(err.Error(), "unknown environment") {
		t.Errorf("unknown env: %v", err)
	}
}

func TestNewWhenSecretsFail(t *testing.T) {
	h := newHarness(t)
	h.must("", "secrets", "set", "BROKEN", "keychain:missing", "--no-sync")
	if out := h.must("", "new", "dev"); !strings.Contains(out, "Installed 0 secret(s); fix the ones above") {
		t.Fatalf("out:\n%s", out)
	}
	h.world.sshFail = true
	if _, err := h.run("", "secrets", "sync", "dev"); err == nil {
		t.Error("sync should fail when ssh does")
	}
}

func TestRebuild(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "") // without a code disk; see TestCodeDisk
	h.must("", "new", "dev", "--secret", "OWN=kept")
	h.world.calls = nil
	out := h.must("", "rebuild", "dev")
	if !h.world.called("limactl delete --force dev") || !h.world.called("limactl start --name=dev") || !strings.Contains(out, "dev was rebuilt") {
		t.Fatalf("rebuild: %s\n%v", out, h.world.calls)
	}
	if !strings.Contains(h.read("secrets.dev.env"), "OWN=kept") {
		t.Error("rebuild should keep the VM's secrets")
	}
	h.world.unsaved = "~/code/x: stashes\n"
	if _, err := h.run("", "rebuild", "dev"); err == nil {
		t.Error("rebuild with unsaved work should fail")
	}
	h.must("", "rebuild", "dev", "--force")
	if _, err := h.run("", "rebuild", "nope"); err == nil {
		t.Error("rebuild of a missing VM should fail")
	}
	h.world.unsaved = ""
	h.world.sshFail = true
	if _, err := h.run("", "rebuild", "dev"); err == nil || !strings.Contains(err.Error(), "unsaved work") {
		t.Errorf("unsaved check failing: %v", err)
	}
}

func TestConfirmations(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	if out, err := h.runTTY([]string{"n"}, "rm", "dev"); err != nil || !strings.Contains(out, "Kept dev") || h.world.vms["dev"] == "" {
		t.Fatalf("rm, answered no: %v\n%s", err, out)
	}
	if out, err := h.runTTY([]string{"n"}, "rebuild", "dev"); err != nil || !strings.Contains(out, "Kept dev") {
		t.Fatalf("rebuild, answered no: %v\n%s", err, out)
	}
	if _, err := h.runTTY([]string{"y"}, "rebuild", "dev"); err != nil || !h.world.called("limactl start --name=dev") {
		t.Fatalf("rebuild, answered yes: %v", err)
	}
	if _, err := h.runTTY([]string{"y"}, "rm", "dev"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.world.vms["dev"]; ok {
		t.Fatal("rm answered yes didn't delete")
	}
}

func TestNewForm(t *testing.T) {
	h := newHarness(t)
	t.Setenv("FORM_VALUE", "from-env")
	// Name; then the secrets step: add a new secret (env var), just this VM.
	forms := [][]string{
		{"formvm", ui.Enter}, // Name
		{"y"},                // Add a new secret?
		{"FORM_SECRET", ui.Enter, ui.Down, ui.Down, ui.Enter, "FORM_VALUE", ui.Enter, "n"}, // env var, just this VM
		{"n"}, // Add another secret?
	}
	out, err := h.runForms(forms, "new")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := h.read("secrets.formvm.env"); !strings.Contains(got, "FORM_SECRET=env:FORM_VALUE") {
		t.Fatalf("VM file = %q\n%s", got, out)
	}
	if !strings.Contains(h.world.sshStdin["formvm"], "export FORM_SECRET='from-env'") {
		t.Fatalf("installed = %q", h.world.sshStdin["formvm"])
	}

	// A second VM is offered the first VM's secret, and a pasted one for
	// every VM goes to the Keychain.
	forms = [][]string{
		{"second", ui.Enter},
		{ui.Space, ui.Enter, "y"},                               // pick FORM_SECRET from formvm; add a new secret
		{"PASTED", ui.Enter, ui.Enter, "s3cret", ui.Enter, "y"}, // pasted value, every VM
		{"n"},
	}
	if out, err := h.runForms(forms, "new"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := h.read("secrets.second.env"); !strings.Contains(got, "FORM_SECRET=env:FORM_VALUE") {
		t.Errorf("second VM didn't get the picked secret: %q", got)
	}
	if h.world.keychain["limabean-secret-PASTED"] != "s3cret" || !strings.Contains(h.read("secrets.env"), "PASTED=keychain:limabean-secret-PASTED") {
		t.Errorf("pasted shared secret: %v / %q", h.world.keychain, h.read("secrets.env"))
	}

	// --secret skips the secrets step.
	if out, err := h.runTTY(nil, "new", "third", "--secret", "X=1"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// --- secrets -----------------------------------------------------------

func TestSecretsForms(t *testing.T) {
	h := newHarness(t)
	// No arguments: the form. A command source, for every VM.
	keys := []string{"CMD_SECRET", ui.Enter, ui.Down, ui.Down, ui.Down, ui.Enter, "echo hi", ui.Enter}
	if out, err := h.runTTY(keys, "secrets", "set"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(h.read("secrets.env"), "CMD_SECRET=cmd:echo hi") {
		t.Fatalf("shared file = %q", h.read("secrets.env"))
	}
	// A 1Password reference, through the form.
	keys = []string{"OP_SECRET", ui.Enter, ui.Down, ui.Enter, "op://v/i/f", ui.Enter}
	if _, err := h.runTTY(keys, "secrets", "set"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.read("secrets.env"), "OP_SECRET=op://v/i/f") {
		t.Fatalf("shared file = %q", h.read("secrets.env"))
	}
	// A name without a reference: the hidden prompt.
	if _, err := h.runTTY([]string{"hidden-value", ui.Enter}, "secrets", "set", "HIDDEN", "--vm", "dev", "--no-sync"); err != nil {
		t.Fatal(err)
	}
	if h.world.keychain["limabean-secret-dev-HIDDEN"] != "hidden-value" {
		t.Fatalf("keychain = %v", h.world.keychain)
	}
}

func TestSecretsSetErrors(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"secrets", "set"},                      // no name, no terminal
		{"secrets", "set", "1BAD", "x"},         // bad name with a reference
		{"secrets", "set", "1BAD"},              // bad name without one
		{"secrets", "set", "OK", "--vm", "b_d"}, // bad VM name
	} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("%v should fail", args)
		}
	}
	if _, err := h.run("", "secrets", "set", "EMPTY", "--from-stdin"); err == nil {
		t.Error("an empty stdin should fail")
	}
	if _, err := h.run("\n", "secrets", "set", "EMPTY", "--from-stdin"); err == nil {
		t.Error("an empty value should fail")
	}
}

func TestSecretsLsAndRmPerVM(t *testing.T) {
	h := newHarness(t)
	if out := h.must("", "secrets", "ls"); !strings.Contains(out, "No secrets yet") {
		t.Errorf("empty ls: %s", out)
	}
	h.must("", "secrets", "set", "SHARED", "a", "--no-sync")
	h.must("", "secrets", "set", "OWN", "b", "--vm", "dev", "--no-sync")
	h.must("", "secrets", "set", "OTHER", "c", "--vm", "zed", "--no-sync")
	if err := os.WriteFile(filepath.Join(h.dir, "secrets.env"), []byte(h.read("secrets.env")+"not a line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := h.must("", "secrets", "ls", "--vm", "dev")
	if !strings.Contains(out, "dev only") || !strings.Contains(out, "every VM") || strings.Contains(out, "OTHER") || !strings.Contains(out, "expected NAME=reference") {
		t.Errorf("ls --vm dev:\n%s", out)
	}
	if out := h.must("", "secrets", "ls"); strings.Index(out, "SHARED") > strings.Index(out, "OWN") {
		t.Errorf("shared secrets should list first:\n%s", out)
	}
	h.must("", "secrets", "rm", "OWN", "--vm", "dev", "--no-sync")
	if strings.Contains(h.read("secrets.dev.env"), "OWN=") {
		t.Error("rm --vm didn't remove it")
	}
}

func TestSecretsEdit(t *testing.T) {
	h := newHarness(t)
	editor := filepath.Join(t.TempDir(), "editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\necho 'EDITED=yes' >> \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", editor)
	out := h.must("", "secrets", "edit")
	if !strings.Contains(h.read("secrets.env"), "EDITED=yes") || !strings.Contains(out, "1 secret(s)") || !strings.Contains(out, "Later: limabean secrets sync") {
		t.Fatalf("edit: %s / %q", out, h.read("secrets.env"))
	}
	h.world.vms["dev"] = "Running"
	if _, err := h.runTTY([]string{"y"}, "secrets", "edit", "--vm", "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.world.sshStdin["dev"], "export EDITED='yes'") {
		t.Errorf("edit then sync: %q", h.world.sshStdin["dev"])
	}
	if _, err := h.run("", "secrets", "edit", "--vm", "b_d"); err == nil {
		t.Error("bad VM name should fail")
	}
	t.Setenv("VISUAL", "false")
	if _, err := h.run("", "secrets", "edit"); err == nil {
		t.Error("a failing editor should fail")
	}
}

func TestSyncListsStoppedVMs(t *testing.T) {
	h := newHarness(t)
	if out := h.must("", "secrets", "sync"); !strings.Contains(out, "No VMs yet") {
		t.Errorf("no VMs: %s", out)
	}
	h.world.vms["up"], h.world.vms["down"] = "Running", "Stopped"
	out := h.must("", "secrets", "sync")
	if !strings.Contains(out, "Stopped, so not updated: down") || !strings.Contains(out, "up: 0 secret(s)") {
		t.Errorf("sync:\n%s", out)
	}
	h.must("", "secrets", "sync", "down") // starts it
	if h.world.vms["down"] != "Running" {
		t.Error("sync <name> should start a stopped VM")
	}
	if _, err := h.run("", "secrets", "sync", "nope"); err == nil {
		t.Error("sync to a missing VM should fail")
	}
}

// --- github, claude -----------------------------------------------------

func TestGitHubMore(t *testing.T) {
	h := newHarness(t)
	if out := h.must("", "github", "status"); !strings.Contains(out, "No GitHub token saved") {
		t.Errorf("status without a token: %s", out)
	}
	if out := h.must("", "github", "sync"); !strings.Contains(out, "No GitHub token saved") {
		t.Errorf("sync without a token: %s", out)
	}
	if _, err := h.run("", "github", "logout"); err == nil {
		t.Error("logout without a token should fail")
	}
	if _, err := h.run("", "github", "login", "--with-token"); err == nil {
		t.Error("login with empty stdin should fail")
	}
	if _, err := h.run("   \n", "github", "login"); err == nil {
		t.Error("login with a blank token should fail")
	}
	// The hidden prompt in a terminal.
	if out, err := h.runTTY([]string{"github_pat_good", ui.Enter}, "github", "login", "--no-sync"); err != nil || h.world.keychain["limabean-github-token"] != "github_pat_good" {
		t.Fatalf("interactive login: %v\n%s", err, out)
	}
	if _, err := h.runTTY(nil, "github", "login", "--with-token"); err == nil {
		t.Error("--with-token in a terminal should ask for a pipe")
	}
	h.world.vms["up"], h.world.vms["down"] = "Running", "Stopped"
	out := h.must("", "github", "status")
	if !strings.Contains(out, "down") || !strings.Contains(out, "stopped") || !strings.Contains(out, "missing") {
		t.Errorf("status:\n%s", out)
	}
	h.world.keychain["limabean-github-token"] = "revoked"
	if out := h.must("", "github", "status"); !strings.Contains(out, "doesn't work") {
		t.Errorf("revoked token:\n%s", out)
	}
}

func TestClaudeSync(t *testing.T) {
	h := newHarness(t)
	if out := h.must("", "claude", "sync"); !strings.Contains(out, "No VMs to register") {
		t.Errorf("no VMs: %s", out)
	}
	h.world.vms["dev"] = "Running"
	if out := h.must("", "claude", "sync"); !strings.Contains(out, "dev (limabean)") {
		t.Errorf("sync: %s", out)
	}
	if err := os.MkdirAll(filepath.Dir(h.claude), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.claude, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.run("", "claude", "sync"); err == nil {
		t.Error("invalid settings should fail")
	}
	t.Setenv("LIMABEAN_CLAUDE_DESKTOP", "no")
	if _, err := h.run("", "claude", "sync"); err == nil {
		t.Error("sync with Claude Desktop off should fail")
	}
}

// --- setup ---------------------------------------------------------------

func TestDoctorBranches(t *testing.T) {
	h := newHarness(t)
	h.writeSSHConfig("Host *\n  User x\nInclude ~/.lima/*/ssh.config\n")
	t.Setenv("FAKE_SSH_ADD", "1")
	t.Setenv("UNSET_FOR_DOCTOR", "")
	os.Unsetenv("UNSET_FOR_DOCTOR")
	h.must("", "secrets", "set", "FROM_OP", "op://v/i/f", "--no-sync")
	h.must("", "secrets", "set", "FROM_ENV", "env:UNSET_FOR_DOCTOR", "--no-sync")
	h.world.keychain["limabean-github-token"] = "revoked"
	if err := os.MkdirAll(filepath.Dir(h.claude), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.claude, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := h.run("", "doctor")
	if err == nil {
		t.Fatalf("doctor should fail:\n%s", out)
	}
	for _, want := range []string{
		"sits inside the Host/Match block", "the agent has no keys", "no config or .gitconfig",
		"settings file isn't valid JSON", "the saved token doesn't work", "$UNSET_FOR_DOCTOR isn't set",
		"not created yet", "Fix the items marked",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor missing %q:\n%s", want, out)
		}
	}
	if !hasProgram("op") && !strings.Contains(out, "needs the 1Password CLI") {
		t.Errorf("op isn't installed here, doctor should say so:\n%s", out)
	}

	// Everything fixed.
	h2 := newHarness(t)
	h2.writeSSHConfig("Include ~/.lima/*/ssh.config\n\nHost *\n")
	gitDir := filepath.Join(h2.home(), ".config", "git")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[user]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_SSH_ADD", "0")
	h2.world.keychain["limabean-github-token"] = "github_pat_good"
	h2.must("", "secrets", "set", "OK", "plain", "--no-sync")
	h2.must("", "init") // writes the config file
	out = h2.must("", "doctor")
	for _, want := range []string{"Include ~/.lima/*/ssh.config (line 1)", "1 key(s) loaded", "token for octocat", "all resolvable", "All good"} {
		if !strings.Contains(out, want) {
			t.Errorf("healthy doctor missing %q:\n%s", want, out)
		}
	}

	// No Include, no agent, Claude off, AWS dir missing.
	h3 := newHarness(t)
	t.Setenv("FAKE_SSH_ADD", "2")
	t.Setenv("LIMABEAN_CLAUDE_DESKTOP", "no")
	t.Setenv("LIMABEAN_AWS_DIR", filepath.Join(h3.home(), "no-aws"))
	out, _ = h3.run("", "doctor")
	for _, want := range []string{"doesn't include Lima's host entries", "no ssh-agent reachable", "off (CLAUDE_DESKTOP", "doesn't exist, so nothing is mounted"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor missing %q:\n%s", want, out)
		}
	}
}

func TestInitInteractive(t *testing.T) {
	h := newHarness(t)
	// ~/.gitconfig symlinked into a dotfiles repo: init suggests that directory.
	dots := filepath.Join(h.home(), "dotfiles", "git")
	if err := os.MkdirAll(dots, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dots, ".gitconfig"), []byte("[user]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dots, ".gitconfig"), filepath.Join(h.home(), ".gitconfig")); err != nil {
		t.Fatal(err)
	}
	h.writeSSHConfig("Host *\n  ServerAliveInterval 60\n")
	// Keep both suggestions, add the Include, save a token now.
	out, err := h.runForms([][]string{{ui.Enter, ui.Enter, "y", "y"}, {"github_pat_good", ui.Enter}}, "init")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if cfg := h.read("config"); !strings.Contains(cfg, `GIT_CONFIG_DIR="~/dotfiles/git"`) {
		t.Errorf("config = %q", cfg)
	}
	ssh, _ := os.ReadFile(filepath.Join(h.home(), ".ssh", "config"))
	if !strings.HasPrefix(string(ssh), "Include ~/.lima/*/ssh.config\n") {
		t.Errorf("ssh config = %q", ssh)
	}
	if _, err := os.Stat(filepath.Join(h.home(), ".ssh", "config.limabean.bak")); err != nil {
		t.Error("no backup of the ssh config")
	}
	if h.world.keychain["limabean-github-token"] != "github_pat_good" {
		t.Error("token not saved")
	}

	// Declining the Include.
	h2 := newHarness(t)
	if out, err := h2.runTTY([]string{ui.Enter, ui.Enter, "n", "n"}, "init"); err != nil || !strings.Contains(out, "SSH config not changed") {
		t.Fatalf("decline: %v\n%s", err, out)
	}
	if guessGitConfigDir("/nowhere", "~/keep") != "~/keep" {
		t.Error("no ~/.gitconfig: keep the current setting")
	}
}

func TestAddSSHIncludeCreatesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "config")
	if err := addSSHInclude(path); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !strings.HasPrefix(string(got), "Include ~/.lima/*/ssh.config") {
		t.Fatalf("got %q", got)
	}
}

// --- plumbing --------------------------------------------------------------

func TestCompletion(t *testing.T) {
	h := newHarness(t)
	for shell, want := range map[string]string{"bash": "bash completion", "zsh": "#compdef", "fish": "complete -c limabean", "powershell": "Register-ArgumentCompleter"} {
		if out := h.must("", "completion", shell); !strings.Contains(out, want) {
			t.Errorf("%s completion missing %q", shell, want)
		}
	}
	if _, err := h.run("", "completion", "tcsh"); err == nil {
		t.Error("unknown shell should fail")
	}
}

func TestCompletionFunctions(t *testing.T) {
	h := newHarness(t)
	h.world.vms["dev"] = "Running"
	h.must("", "secrets", "set", "A", "x", "--no-sync")
	a := NewApp("test", os.DirFS(filepath.Join("..", "..")))
	a.configPath = filepath.Join(h.dir, "config")
	a.Lima = &lima.Client{Runner: h.world}
	if got, _ := a.completeVMs(nil, nil, ""); len(got) != 1 || !strings.HasPrefix(got[0], "dev\t") {
		t.Errorf("completeVMs = %v", got)
	}
	if got, _ := a.completeVMs(nil, []string{"dev"}, ""); got != nil {
		t.Errorf("second arg: %v", got)
	}
	if got, _ := completeEnvs(a)(nil, []string{"dev"}, ""); len(got) == 0 || got[0] != "scratch" {
		t.Errorf("completeEnvs = %v", got)
	}
	if got, _ := completeEnvs(a)(nil, nil, ""); got != nil {
		t.Errorf("completeEnvs first arg = %v", got)
	}
	if got, _ := completeSecretNames(a)(nil, nil, ""); len(got) != 1 || got[0] != "A" {
		t.Errorf("completeSecretNames = %v", got)
	}
	if got, _ := completeSecretNames(a)(nil, []string{"A"}, ""); got != nil {
		t.Errorf("completeSecretNames second arg = %v", got)
	}
}

func TestExecuteExitCodes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if code := Execute("test", fstest.MapFS{}, []string{"version"}); code != 0 {
		t.Errorf("version exit = %d", code)
	}
	if code := Execute("test", fstest.MapFS{}, []string{"no-such-command"}); code != 1 {
		t.Errorf("unknown command exit = %d", code)
	}
	if code := Execute("test", fstest.MapFS{}, []string{"--config", t.TempDir(), "version"}); code != 1 {
		t.Errorf("a directory as config should fail with 1, got %d", code)
	}
}

func TestSetupDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("LIMABEAN_KEYCHAIN_PREFIX", "")
	a := NewApp("test", fstest.MapFS{})
	a.plain = true
	if err := a.setup(); err != nil {
		t.Fatal(err)
	}
	if a.keychainPrefix != "limabean" || !strings.HasSuffix(a.stateDir, filepath.Join(".local", "state", "limabean")) ||
		!strings.HasSuffix(cacheDir(), filepath.Join(".cache", "limabean")) || a.Lima == nil || a.Keychain == nil || a.Resolver.Keychain == nil {
		t.Fatalf("defaults: %+v", a)
	}
	if a.secretService("vm", "N") != "limabean-secret-vm-N" || a.ownsKeychainRef("keychain:other") {
		t.Error("service naming")
	}
	if envOr("LIMABEAN_TEST_UNSET_VAR", "def") != "def" || firstNonEmpty("", "") != "" || padRight("long-name", 3) != "long-name " {
		t.Error("helpers")
	}
	if !hasProgram("sh") || hasProgram("definitely-not-a-program-xyz") {
		t.Error("hasProgram")
	}
}

// Claude Desktop errors on a connection to a stopped VM, so only running VMs
// are listed: stop removes the entry, starting adds it back, and ls and sync
// catch up with VMs started or stopped with limactl.
func TestClaudeFollowsRunningVMs(t *testing.T) {
	h := newHarness(t)
	entries := func() string {
		names, _ := claude.Settings{Path: h.claude}.VMs()
		return strings.Join(names, ",")
	}
	h.world.vms["dev"] = "Running"
	h.must("", "claude", "sync")
	if entries() != "dev" {
		t.Fatalf("after sync: %q", entries())
	}

	h.must("", "stop", "dev")
	if entries() != "" {
		t.Fatalf("stop kept the entry: %q", entries())
	}
	if out := h.must("", "ssh", "dev", "true"); !strings.Contains(out, "Added to Claude Desktop") || entries() != "dev" {
		t.Fatalf("starting with ssh didn't add the entry: %q\n%s", entries(), out)
	}

	// Stopped behind limabean's back, then started the same way.
	h.world.vms["dev"] = "Stopped"
	if out := h.must("", "ls"); !strings.Contains(out, "Removed dev from Claude Desktop (the VM is stopped)") || entries() != "" {
		t.Fatalf("ls didn't remove the stopped VM: %q\n%s", entries(), out)
	}
	if out := h.must("", "claude", "sync"); !strings.Contains(out, "dev is stopped") || entries() != "" {
		t.Fatalf("sync registered a stopped VM: %q\n%s", entries(), out)
	}
	h.world.vms["dev"] = "Running"
	if out := h.must("", "ls"); !strings.Contains(out, "Added dev to Claude Desktop (the VM is running)") || entries() != "dev" {
		t.Fatalf("ls didn't add the running VM: %q\n%s", entries(), out)
	}

	// start boots it and adds the entry; a second start just ensures it.
	h.world.vms["dev"] = "Stopped"
	h.must("", "ls")
	if out := h.must("", "start", "dev"); !strings.Contains(out, "Started dev") || h.world.vms["dev"] != "Running" || entries() != "dev" {
		t.Fatalf("start: %q %v\n%s", entries(), h.world.vms, out)
	}
	if out := h.must("", "start", "dev"); !strings.Contains(out, "already running") {
		t.Fatalf("second start: %s", out)
	}
	if _, err := h.run("", "start", "nope"); err == nil {
		t.Error("starting a missing VM should fail")
	}

	// An entry left from before this change goes when stop finds the VM
	// already stopped.
	h.world.vms["dev"] = "Stopped"
	h.must("", "stop", "dev")
	if entries() != "" {
		t.Fatalf("stopping a stopped VM kept the entry: %q", entries())
	}
}
