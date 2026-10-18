package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rossedman/limabean/internal/templates"
)

func TestNewWithPacks(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	t.Setenv("LIMABEAN_PACKS", "go,python,node,kubernetes,infra,containers,db") // the old default, as init wrote it
	out := h.must("", "new", "api", "--with", "rust,java", "--without", "kubernetes")
	if !strings.Contains(out, "Packs: core, containers, db, go, infra, java, node, python, rust") {
		t.Fatalf("new --with --without:\n%s", out)
	}
	calls := strings.Join(h.world.calls, "\n")
	for _, want := range []string{
		`"path": "/etc/limabean/mise/50-rust.toml", "content": "# Rust`,
		`"path": "/etc/limabean/mise/50-java.toml"`,
		`"path": "/etc/limabean/apt/db.list", "content": "postgresql-client\nsqlite3\nredis-tools\n"`,
		`"path": "/etc/limabean/apt/containers.list"`,
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("limactl start is missing %s", want)
		}
	}
	for _, unwanted := range []string{"50-kubernetes.toml", "50-core.toml", "50-ruby.toml"} {
		if strings.Contains(calls, unwanted) {
			t.Errorf("limactl start shouldn't have %s", unwanted)
		}
	}

	var vms []lsEntry
	if err := json.Unmarshal([]byte(h.must("", "ls", "--json")), &vms); err != nil || strings.Join(vms[0].Packs, ",") != "containers,db,go,infra,java,node,python,rust" {
		t.Fatalf("ls --json packs: %v %+v", err, vms)
	}

	// Rebuild keeps the VM's packs, and can change them.
	h.world.calls = nil
	h.must("", "rebuild", "api", "--force", "--with", "ruby", "--without", "java")
	calls = strings.Join(h.world.calls, "\n")
	if !strings.Contains(calls, "50-rust.toml") || !strings.Contains(calls, "50-ruby.toml") || strings.Contains(calls, "50-java.toml") || strings.Contains(calls, "50-kubernetes.toml") {
		t.Fatalf("rebuild should keep rust, add ruby, drop java, and not bring kubernetes back: %v", h.world.calls)
	}

	// A VM made before packs gets the defaults when rebuilt.
	os.Remove(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "api.packs"))
	h.world.calls = nil
	h.must("", "rebuild", "api", "--force")
	if calls := strings.Join(h.world.calls, "\n"); !strings.Contains(calls, "50-kubernetes.toml") || strings.Contains(calls, "50-rust.toml") {
		t.Errorf("an old VM should rebuild with the default packs: %v", h.world.calls)
	}
	if err := json.Unmarshal([]byte(h.must("", "ls", "--json")), &vms); err != nil || vms[0].Packs == nil {
		t.Errorf("rebuild should record the packs: %+v", vms)
	}
	os.Remove(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "api.packs"))
	if err := json.Unmarshal([]byte(h.must("", "ls", "--json")), &vms); err != nil || vms[0].Packs != nil {
		t.Errorf("unknown packs are null in ls --json: %+v", vms)
	}

	// PACKS changes the defaults; empty means core only.
	t.Setenv("LIMABEAN_PACKS", "")
	h.world.calls = nil
	h.must("", "new", "bare")
	if strings.Contains(strings.Join(h.world.calls, "\n"), "/etc/limabean/mise/50-") {
		t.Errorf("PACKS empty should give core only: %v", h.world.calls)
	}
	t.Setenv("LIMABEAN_PACKS", "go")

	for _, args := range [][]string{
		{"new", "x", "--with", "cobol"},
		{"new", "x", "--without", "core"},
		{"new", "x", "--with", "core"},
		{"rebuild", "bare", "--force", "--with", "nope"},
	} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("limabean %v should fail", args)
		}
	}
	if _, ok := h.world.vms["x"]; ok {
		t.Error("an unknown pack should stop new before it creates anything")
	}

	// rm forgets them.
	h.must("", "rm", "bare", "--force")
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "limabean", "bare.packs")); !os.IsNotExist(err) {
		t.Errorf("rm left the packs: %v", err)
	}
}

func TestPacksCommand(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_PACKS", "go,rust")
	out := h.must("", "packs")
	for _, want := range []string{"PACK", "core", "always", "ripgrep", "rust", "cargo-binstall", "db", "postgresql-client"} {
		if !strings.Contains(out, want) {
			t.Errorf("packs missing %q:\n%s", want, out)
		}
	}
	var packs []packEntry
	if err := json.Unmarshal([]byte(h.must("", "packs", "--json")), &packs); err != nil {
		t.Fatal(err)
	}
	defaults := map[string]bool{}
	for _, p := range packs {
		if p.Description == "" {
			t.Errorf("%s has no description", p.Name)
		}
		defaults[p.Name] = p.Default
	}
	if !defaults["core"] || !defaults["go"] || !defaults["rust"] || defaults["node"] || defaults["java"] {
		t.Errorf("defaults with PACKS=go,rust: %v", defaults)
	}
}

func TestNewDefaultsToCore(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	out := h.must("", "new", "bare")
	if !strings.Contains(out, "Packs: core only (more later: limabean rebuild bare --with go,node)") {
		t.Fatalf("out:\n%s", out)
	}
	if strings.Contains(strings.Join(h.world.calls, "\n"), "/etc/limabean/mise/50-") {
		t.Errorf("a pack was added: %v", h.world.calls)
	}
}

func TestPackChoice(t *testing.T) {
	with, without := packChoice([]string{"go", "node"}, []string{"go", "rust"})
	if strings.Join(with, ",") != "go,rust" || strings.Join(without, ",") != "node" {
		t.Errorf("packChoice = %v, %v", with, without)
	}
	all, err := templates.Packs(filepath.Join(repoRoot, "templates"))
	if err != nil {
		t.Fatal(err)
	}
	opts := packOptions(all, []string{"go"})
	for _, o := range opts {
		if o.Value == "core" {
			t.Error("core is offered")
		}
	}
	if len(opts) != len(all)-1 {
		t.Errorf("options: %d of %d packs", len(opts), len(all))
	}
}
