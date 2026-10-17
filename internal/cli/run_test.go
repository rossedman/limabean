package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.world.ghq = "/home/me.linux/code/github.com/me/app\n"
	out := h.must("", "run", "--rm", "--name", "job", "--with", "claude", "--repo", "me/app", "--", "go", "test", "./...")
	calls := strings.Join(h.world.calls, "\n")
	for _, want := range []string{"limactl start --name=job", "50-claude.toml", "lima-job ghq get 'me/app'", "attach ssh job cd '/home/me.linux/code/github.com/me/app' && 'go' 'test' './...'", "limactl delete --force job"} {
		if !strings.Contains(calls, want) {
			t.Errorf("calls missing %q:\n%s", want, calls)
		}
	}
	if _, ok := h.world.vms["job"]; ok || !strings.Contains(out, "Deleted job") {
		t.Errorf("--rm should delete the VM:\n%s", out)
	}

	// A failing command: the VM is still deleted, and its status is limabean's.
	h.world.attachErr = exec.Command("sh", "-c", "exit 7").Run()
	_, err := h.run("", "run", "--rm", "--", "make test && echo ok")
	var code exitCode
	if !errors.As(err, &code) || code != 7 {
		t.Fatalf("exit status: %v", err)
	}
	if !h.world.called("attach ssh run-") || !strings.Contains(strings.Join(h.world.calls, "\n"), " make test && echo ok") {
		t.Errorf("one argument runs as it is: %v", h.world.calls)
	}
	if len(h.world.vms) != 0 {
		t.Errorf("a failed run should still delete its VM: %v", h.world.vms)
	}
	h.world.attachErr = errors.New("ssh: lost connection")
	if _, err := h.run("", "run", "--rm", "--", "x"); err == nil || errors.As(err, &code) {
		t.Errorf("an ssh failure: %v", err)
	}
	h.world.attachErr = nil
	h.world.failDelete = true
	if out, err := h.run("", "run", "--rm", "--name", "stuck", "--", "true"); err != nil || !strings.Contains(out, "rm stuck --force") {
		t.Errorf("a VM that won't delete: %v\n%s", err, out)
	}
	h.world.failDelete = false
	delete(h.world.vms, "stuck")

	// Kept, with a TTL.
	out = h.must("", "run", "--name", "kept", "--ttl", "2h", "--", "true")
	if h.world.vms["kept"] != "Running" || !strings.Contains(out, "kept is deleted at") || !strings.Contains(out, "Nothing deletes expired VMs yet") || !strings.Contains(out, "The VM is kept") {
		t.Errorf("kept:\n%s", out)
	}
	for _, args := range [][]string{
		{"run", "--name", "kept", "--", "x"},
		{"run", "--name", "bad_name", "--", "x"},
		{"run", "--rm", "--ttl", "1h", "--", "x"},
		{"run", "--with", "cobol", "--", "x"},
		{"run", "--secret", "nope", "--", "x"},
		{"run", "--repo", "--evil", "--", "x"},
		{"run", "--ttl", "5s", "--", "x"},
	} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("%v should fail", args)
		}
	}
}

func TestTTL(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.must("", "new", "spike", "--ttl", "2h")
	h.must("", "new", "keep")
	var vms []lsEntry
	if err := json.Unmarshal([]byte(h.must("", "ls", "--json")), &vms); err != nil {
		t.Fatal(err)
	}
	exp, err := time.Parse(time.RFC3339, vms[1].Expires)
	if vms[1].Name != "spike" || err != nil || time.Until(exp) < time.Hour || vms[0].Expires != "" {
		t.Fatalf("ls --json expires: %+v %v", vms, err)
	}
	if out, _ := h.runTTY(nil, "ls"); !strings.Contains(out, "spike is deleted in 1h59m") && !strings.Contains(out, "spike is deleted in 2h00m") {
		t.Errorf("ls in a terminal:\n%s", out)
	}
	if out := h.must("", "ls"); strings.Contains(out, "deleted in") {
		t.Errorf("plain ls is for scripts:\n%s", out)
	}

	h.must("", "rm", "--expired")
	if _, ok := h.world.vms["spike"]; !ok {
		t.Fatal("spike hasn't expired yet")
	}
	state := filepath.Join(filepath.Dir(h.dir), "state", "limabean", "spike.expires")
	if err := os.WriteFile(state, []byte(time.Now().Add(-time.Minute).Format(time.RFC3339)), 0o644); err != nil {
		t.Fatal(err)
	}
	h.world.unsaved = "~/code/x: uncommitted changes\n" // not checked: a TTL means delete
	h.world.activity["keep"] = "1000 990 load average 2"
	out := h.must("", "autostop", "run", "--idle", "2h")
	if _, ok := h.world.vms["spike"]; ok || !strings.Contains(out, "Deleted spike (expired)") || fileExists(state) {
		t.Errorf("autostop run should delete the expired VM:\n%s", out)
	}
	if _, ok := h.world.vms["keep"]; !ok {
		t.Error("a VM without a TTL is never deleted")
	}
	if _, err := h.run("", "rm", "keep", "--expired"); err == nil {
		t.Error("rm --expired with a name should fail")
	}
	if _, err := h.run("", "autostop", "run", "--idle", "1s"); err == nil {
		t.Error("a tiny --idle should fail")
	}
	if _, err := h.run("", "new", "x", "--ttl", "1s"); err == nil {
		t.Error("a tiny --ttl should fail")
	}

	h.must("", "new", "doomed", "--ttl", "1h")
	if err := os.WriteFile(filepath.Join(filepath.Dir(h.dir), "state", "limabean", "doomed.expires"), []byte(time.Now().Add(-time.Minute).Format(time.RFC3339)), 0o644); err != nil {
		t.Fatal(err)
	}
	h.world.failDelete = true
	if _, err := h.run("", "rm", "--expired"); err == nil {
		t.Error("a VM that won't delete should fail the run")
	}
}
