package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rossedman/limabean/internal/ui"
)

// mcpClient connects an MCP client to limabean's server over an in-memory
// transport, against the fake world.
func (h *harness) mcpClient(allowDestructive bool) (*mcp.ClientSession, *bytes.Buffer) {
	h.t.Helper()
	var stdout bytes.Buffer
	a := h.app(ui.NewPlain(&stdout))
	a.configPath = filepath.Join(h.dir, "config")
	if err := a.setup(); err != nil {
		h.t.Fatal(err)
	}
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := a.mcpServer(allowDestructive).Connect(ctx, st, nil); err != nil {
		h.t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { cs.Close() })
	return cs, &stdout
}

// call calls a tool and returns its text and whether it reported an error.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
		if tool.Description == "" || tool.Annotations == nil || tool.Annotations.Title == "" {
			t.Errorf("%s: needs a description and a title", tool.Name)
		}
	}
	slices.Sort(names)
	return names
}

func TestMCPTools(t *testing.T) {
	h := newHarness(t)
	cs, stdout := h.mcpClient(false)

	safe := []string{"clone_repo", "clone_vm", "create_vm", "list_ports", "list_vms", "network_log", "project_down", "project_init", "project_logs", "project_plan", "project_run", "project_status", "project_up", "run_in_vm", "snapshot_list", "snapshot_save", "start_vm", "stop_vm"}
	if got := toolNames(t, cs); !slices.Equal(got, safe) {
		t.Fatalf("tools without --allow-destructive: %v", got)
	}

	if out, isErr := call(t, cs, "list_vms", nil); isErr || strings.TrimSpace(out) != "[]" {
		t.Fatalf("list_vms with no VMs: %q %v", out, isErr)
	}
	out, isErr := call(t, cs, "create_vm", map[string]any{"name": "agent", "cpus": 6, "memory": 12})
	if isErr || !strings.Contains(out, "Created agent from scratch") {
		t.Fatalf("create_vm: %v\n%s", isErr, out)
	}
	if !h.world.called("limactl start --name=agent --tty=false --cpus 6 --memory 12") && !strings.Contains(strings.Join(h.world.calls, "\n"), "--cpus 6 --memory 12") {
		t.Errorf("create_vm should pass cpus and memory: %v", h.world.calls)
	}
	// A sandbox: the agent can narrow a new VM's network, never widen it.
	out, isErr = call(t, cs, "create_vm", map[string]any{"name": "sandbox", "allow": []string{"github", "pypi.org"}, "locked": true})
	if isErr || !strings.Contains(out, "Network: 5 domain(s), locked") {
		t.Fatalf("create_vm with allow and locked: %v\n%s", isErr, out)
	}
	if out, isErr := call(t, cs, "create_vm", map[string]any{"name": "rusty", "with": []string{"rust"}}); isErr || !strings.Contains(out, "rust") {
		t.Errorf("create_vm with a pack: %v\n%s", isErr, out)
	}
	if out, isErr := call(t, cs, "create_vm", map[string]any{"name": "bad", "with": []string{"--secret=X=cmd:id"}}); !isErr {
		t.Errorf("a pack name that looks like a flag should be refused:\n%s", out)
	}
	if out, isErr := call(t, cs, "create_vm", map[string]any{"name": "bad", "ttl": "soon"}); !isErr || !strings.Contains(out, "duration") {
		t.Errorf("a bad ttl: %v\n%s", isErr, out)
	}
	if out, isErr := call(t, cs, "create_vm", map[string]any{"name": "brief", "ttl": "90m"}); isErr || !strings.Contains(out, "brief is deleted at") {
		t.Errorf("create_vm with a ttl: %v\n%s", isErr, out)
	}
	call(t, cs, "stop_vm", map[string]any{"name": "brief"})
	delete(h.world.vms, "brief")
	if out, isErr := call(t, cs, "create_vm", map[string]any{"name": "bad", "with": []string{"cobol"}}); !isErr || !strings.Contains(out, "unknown pack") {
		t.Errorf("an unknown pack: %v\n%s", isErr, out)
	}
	call(t, cs, "stop_vm", map[string]any{"name": "rusty"})
	delete(h.world.vms, "rusty")
	if out, isErr := call(t, cs, "create_vm", map[string]any{"name": "bad", "allow": []string{"--secret=X=cmd:id"}}); !isErr || !strings.Contains(out, "isn't a domain") {
		t.Errorf("an allow entry that looks like a flag should be refused: %v\n%s", isErr, out)
	}
	call(t, cs, "stop_vm", map[string]any{"name": "sandbox"})
	delete(h.world.vms, "sandbox")

	var vms []map[string]any
	out, _ = call(t, cs, "list_vms", nil)
	if err := json.Unmarshal([]byte(out), &vms); err != nil || len(vms) != 1 || vms[0]["name"] != "agent" {
		t.Fatalf("list_vms: %v %s", err, out)
	}

	// Commands run in the VM; the exit code and output come back.
	if out, isErr := call(t, cs, "run_in_vm", map[string]any{"name": "agent", "command": "echo hello"}); isErr || out != "exit code 0\nhello\n" {
		t.Errorf("run_in_vm: %q %v", out, isErr)
	}
	if out, isErr := call(t, cs, "run_in_vm", map[string]any{"name": "agent", "command": "false"}); !isErr || !strings.HasPrefix(out, "exit code 3\nit failed") {
		t.Errorf("a failing command: %q %v", out, isErr)
	}
	if out, isErr := call(t, cs, "run_in_vm", map[string]any{"name": "agent", "command": "sleep", "timeout_seconds": 1}); !isErr || !strings.Contains(out, "timed out after 1s") {
		t.Errorf("timeout: %q %v", out, isErr)
	}
	if _, isErr := call(t, cs, "run_in_vm", map[string]any{"name": "agent", "command": "  "}); !isErr {
		t.Error("an empty command should fail")
	}
	if _, isErr := call(t, cs, "run_in_vm", map[string]any{"name": "agent"}); !isErr {
		t.Error("a missing command should fail schema validation")
	}

	call(t, cs, "clone_repo", map[string]any{"name": "agent", "repo": "rossedman/limabean"})
	if !strings.Contains(strings.Join(h.world.calls, "\n"), "lima-agent ghq get 'rossedman/limabean'") {
		t.Errorf("clone_repo: %v", h.world.calls)
	}
	if _, isErr := call(t, cs, "clone_repo", map[string]any{"name": "agent", "repo": "--vcs=evil"}); !isErr {
		t.Error("a repo that looks like a flag should be refused")
	}

	if out, isErr := call(t, cs, "snapshot_save", map[string]any{"name": "agent", "snapshot": "start"}); isErr || !strings.Contains(out, "Saved agent as start") {
		t.Errorf("snapshot_save: %v\n%s", isErr, out)
	}
	if out, _ := call(t, cs, "snapshot_list", map[string]any{"name": "agent"}); !strings.Contains(out, `"tag": "start"`) {
		t.Errorf("snapshot_list: %s", out)
	}
	h.world.ss = ssOutput
	h.world.macPorts[3000] = true
	var ports []vmPort
	out, isErr = call(t, cs, "list_ports", map[string]any{"name": "agent"})
	if err := json.Unmarshal([]byte(out), &ports); err != nil || isErr || len(ports) != 4 || ports[0].Port != 3000 || !ports[0].Forwarded {
		t.Errorf("list_ports: %v %v\n%s", err, isErr, out)
	}

	if out, isErr := call(t, cs, "network_log", map[string]any{"name": "agent"}); !isErr || !strings.Contains(out, "open") {
		t.Errorf("network_log of an open VM: %v\n%s", isErr, out)
	}

	if out, isErr := call(t, cs, "stop_vm", map[string]any{"name": "agent"}); isErr || h.world.vms["agent"] != "Stopped" {
		t.Errorf("stop_vm: %v\n%s", isErr, out)
	}
	if out, isErr := call(t, cs, "run_in_vm", map[string]any{"name": "agent", "command": "echo x"}); !isErr || !strings.Contains(out, "start it with start_vm") {
		t.Errorf("run_in_vm on a stopped VM: %q", out)
	}
	if _, isErr := call(t, cs, "start_vm", map[string]any{"name": "agent"}); isErr || h.world.vms["agent"] != "Running" {
		t.Error("start_vm")
	}

	// Names that would be flags, or worse, never reach the command line.
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"stop_vm", map[string]any{"name": "--help"}},
		{"start_vm", map[string]any{"name": "a b"}},
		{"create_vm", map[string]any{"name": "ok", "env": "../../etc"}},
		{"create_vm", map[string]any{"name": "ok", "cpus": -1}},
		{"run_in_vm", map[string]any{"name": "-x", "command": "echo"}},
		{"snapshot_save", map[string]any{"name": "agent", "snapshot": "--force"}},
		{"snapshot_list", map[string]any{"name": ""}},
		{"list_ports", map[string]any{"name": "-v"}},
		{"stop_vm", map[string]any{"name": "nope"}},
		{"clone_vm", map[string]any{"name": "agent", "new_name": "--force"}},
		{"clone_vm", map[string]any{"name": "agent", "new_name": ""}},
	} {
		if out, isErr := call(t, cs, tc.tool, tc.args); !isErr {
			t.Errorf("%s %v should fail:\n%s", tc.tool, tc.args, out)
		}
	}
	if out, isErr := call(t, cs, "clone_vm", map[string]any{"name": "agent", "new_name": "agent-2"}); isErr || !strings.Contains(out, "Copied agent to agent-2") {
		t.Errorf("clone_vm: %v\n%s", isErr, out)
	}
	if stdout.Len() > 0 {
		t.Errorf("tools printed to the server's stdout, which carries the protocol:\n%s", stdout)
	}
}

func TestMCPDestructiveTools(t *testing.T) {
	h := newHarness(t)
	h.must("", "new", "dev")
	h.must("", "snapshot", "save", "dev", "good")
	cs, _ := h.mcpClient(true)
	names := toolNames(t, cs)
	for _, want := range []string{"delete_vm", "rebuild_vm", "snapshot_restore"} {
		if !slices.Contains(names, want) {
			t.Fatalf("--allow-destructive should add %s: %v", want, names)
		}
	}
	for tool := range cs.Tools(context.Background(), nil) {
		destructive := tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint
		if slices.Contains([]string{"delete_vm", "rebuild_vm", "snapshot_restore", "run_in_vm", "project_run"}, tool.Name) != destructive {
			t.Errorf("%s: destructiveHint = %v", tool.Name, destructive)
		}
		// Claude Code asks before every call to these.
		asks := tool.Meta["anthropic/requiresUserInteraction"] == true
		if slices.Contains([]string{"delete_vm", "rebuild_vm", "snapshot_restore", "project_up"}, tool.Name) != asks {
			t.Errorf("%s: anthropic/requiresUserInteraction = %v", tool.Name, asks)
		}
	}

	h.world.writeFile(filepath.Join("dev", "disk"), "broken")
	if out, isErr := call(t, cs, "snapshot_restore", map[string]any{"name": "dev", "snapshot": "good"}); isErr || h.world.readFile(filepath.Join("dev", "disk")) != "system of dev" {
		t.Fatalf("snapshot_restore: %v\n%s", isErr, out)
	}
	if _, isErr := call(t, cs, "snapshot_restore", map[string]any{"name": "dev", "snapshot": "../x"}); !isErr {
		t.Error("a bad snapshot name should fail")
	}
	if out, isErr := call(t, cs, "rebuild_vm", map[string]any{"name": "dev"}); isErr || !strings.Contains(out, "dev was rebuilt") {
		t.Fatalf("rebuild_vm: %v\n%s", isErr, out)
	}
	// delete_vm keeps rm's unsaved-work check.
	h.world.unsaved = "~/code/x: unpushed commits\n"
	if out, isErr := call(t, cs, "delete_vm", map[string]any{"name": "dev"}); !isErr || !strings.Contains(out, "would be lost") || h.world.vms["dev"] == "" {
		t.Fatalf("delete_vm with unsaved work: %v\n%s", isErr, out)
	}
	h.world.unsaved = ""
	if out, isErr := call(t, cs, "delete_vm", map[string]any{"name": "dev"}); isErr || h.world.vms["dev"] != "" {
		t.Fatalf("delete_vm: %v\n%s", isErr, out)
	}
	for _, tool := range []string{"delete_vm", "rebuild_vm", "snapshot_restore"} {
		if _, isErr := call(t, cs, tool, map[string]any{"name": "--force"}); !isErr {
			t.Errorf("%s --force should be refused", tool)
		}
	}
}

func TestMCPHelpers(t *testing.T) {
	long := bytes.Repeat([]byte("x"), mcpMaxOutput+10)
	if got := tail(long); !strings.HasPrefix(got, "[first 10 bytes cut]\n") || len(got) > mcpMaxOutput+40 {
		t.Errorf("tail of long output: %q…", got[:40])
	}
	if tail([]byte("short")) != "short" {
		t.Error("short output is kept")
	}
}
