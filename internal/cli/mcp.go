package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/secrets"
	"github.com/rossedman/limabean/internal/ui"
)

func newMCPCmd(a *App) *cobra.Command {
	var allowDestructive bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Let Claude manage your VMs, as an MCP server",
		Long: `Run an MCP server on stdin and stdout, so Claude Code or Claude Desktop on
your Mac can manage limabean VMs: list, create, start and stop them, run
commands in them, clone repositories, see forwarded ports and save
snapshots. An agent can then make itself a scratch VM, work in it, and
checkpoint it before something risky. For a checkout with a
.limabean.yaml, the project tools plan, bring up, inspect and stop its VM
as "limabean up" would; Claude Code asks you before every project_up.

What the tools can't do:
  - run anything on your Mac: commands run inside VMs only, and the
    project tools read only a checkout's .limabean.yaml and .git/config
  - read secret values, or add secrets (a cmd: secret would run on the Mac)
  - widen a VM's network, from a project file or otherwise
  - delete, rebuild or restore a VM, unless you pass --allow-destructive.
    Even then, deleting stops at unsaved work as "limabean rm" does, and
    Claude Code asks you before every call to them.

Commands in a VM run as you, with the VM's secrets in their environment,
and can reach anything the VM can, including GitHub with its token.`,
		Example: `  # Claude Code
  claude mcp add limabean -- limabean mcp

  # Claude Desktop: in claude_desktop_config.json
  #   "mcpServers": { "limabean": { "command": "/opt/homebrew/bin/limabean", "args": ["mcp"] } }

  # Also let Claude delete, rebuild and restore VMs
  claude mcp add limabean -- limabean mcp --allow-destructive`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// stdout carries the protocol: anything else goes to stderr.
			a.UI = ui.NewPlain(os.Stderr)
			return a.mcpServer(allowDestructive).Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	cmd.Flags().BoolVar(&allowDestructive, "allow-destructive", false, "also offer tools that delete, rebuild or restore VMs")
	return cmd
}

// Tool inputs. Fields without omitempty are required.
type (
	mcpVM struct {
		Name string `json:"name" jsonschema:"the VM's name, as list_vms shows it"`
	}
	mcpCreate struct {
		Name   string   `json:"name" jsonschema:"name for the new VM: letters, digits and dashes"`
		Env    string   `json:"env,omitempty" jsonschema:"environment template; default scratch"`
		CPUs   int      `json:"cpus,omitempty" jsonschema:"CPUs; default the template's (2)"`
		Memory int      `json:"memory,omitempty" jsonschema:"memory in GiB; default the template's (4)"`
		Disk   int      `json:"disk,omitempty" jsonschema:"system disk in GiB; default the template's (10). ~/code has a disk of its own"`
		Allow  []string `json:"allow,omitempty" jsonschema:"only let the VM reach these domains (and subdomains) or presets: claude, docker, github, go, java, npm, python, ruby, rust, ubuntu"`
		With   []string `json:"with,omitempty" jsonschema:"packs of tools the VM needs besides the core shell tools, Docker and Python: go, node, python, rust, java, ruby, kubernetes, infra, db, claude, … (limabean packs lists them)"`
		Locked bool     `json:"locked,omitempty" jsonschema:"take sudo and Docker away from the VM's user, so the allowlist can't be turned off from inside"`
		TTL    string   `json:"ttl,omitempty" jsonschema:"delete the VM, whatever is in it, once it's this old, e.g. 2h or 90m; for scratch work that should clean itself up"`
	}
	mcpCloneVM struct {
		Name    string `json:"name" jsonschema:"the VM to copy"`
		NewName string `json:"new_name" jsonschema:"name for the copy: letters, digits and dashes"`
	}
	mcpRun struct {
		Name    string `json:"name" jsonschema:"the VM to run in; it must be running"`
		Command string `json:"command" jsonschema:"shell command, run by the VM user's login shell in their home directory"`
		Timeout int    `json:"timeout_seconds,omitempty" jsonschema:"give up after this many seconds; default 600, at most 3600"`
	}
	mcpClone struct {
		Name string `json:"name" jsonschema:"the VM to clone into; it must be running"`
		Repo string `json:"repo" jsonschema:"owner/repo, host/owner/repo or a URL; cloned to ~/code/<host>/<owner>/<repo>"`
	}
	mcpSnapshot struct {
		Name string `json:"name" jsonschema:"the VM's name"`
		Tag  string `json:"snapshot,omitempty" jsonschema:"snapshot name; default the date and time when saving, the latest when restoring"`
	}
	mcpProject struct {
		Dir string `json:"dir" jsonschema:"absolute path of the project's checkout on the Mac: the directory that holds its .limabean.yaml"`
	}
	mcpProjectLogs struct {
		Dir   string `json:"dir" jsonschema:"absolute path of the project's checkout on the Mac: the directory that holds its .limabean.yaml"`
		Log   string `json:"log,omitempty" jsonschema:"which log: provision (default; creating the VM), setup, or start"`
		Lines int    `json:"lines,omitempty" jsonschema:"how many lines from the end; default 100, at most 2000"`
	}
)

const (
	mcpDefaultTimeout = 10 * time.Minute
	mcpMaxTimeout     = time.Hour
	mcpMaxOutput      = 100 << 10
)

// mcpServer builds the MCP server and its tools.
func (a *App) mcpServer(allowDestructive bool) *mcp.Server {
	a.mcpMode, a.noRecreate = true, !allowDestructive
	s := mcp.NewServer(&mcp.Implementation{Name: "limabean", Version: a.Version}, &mcp.ServerOptions{
		Instructions: "limabean manages disposable Ubuntu dev VMs on this Mac. Commands run inside VMs, never on the Mac. " +
			"Repositories live in ~/code/<host>/<owner>/<repo>. Save a snapshot before risky changes. " +
			"A checkout with a .limabean.yaml describes its own VM: use the project_ tools with its directory, and show the user project_plan's output before project_up.",
	})
	no, yes := false, true
	var mu sync.Mutex // one limabean command at a time
	cli := func(ctx context.Context, args ...string) (*mcp.CallToolResult, any, error) {
		mu.Lock()
		defer mu.Unlock()
		out, err := a.runCLI(ctx, args...)
		if err != nil {
			return nil, nil, fmt.Errorf("%v\n%s", err, out)
		}
		return text(out), nil, nil
	}
	tool := func(name, title, desc string, readOnly bool, destructive *bool) *mcp.Tool {
		return &mcp.Tool{Name: name, Description: desc, Annotations: &mcp.ToolAnnotations{
			Title: title, ReadOnlyHint: readOnly, DestructiveHint: destructive, OpenWorldHint: &no,
		}}
	}

	// Claude Code asks before every call to a tool with this, whatever its
	// permission mode, with no "don't ask again"; other clients see
	// destructiveHint.
	ask := func(t *mcp.Tool) *mcp.Tool {
		t.Meta = mcp.Meta{"anthropic/requiresUserInteraction": true}
		return t
	}

	mcp.AddTool(s, tool("project_status", "Show a project's VM", "For a checkout with a .limabean.yaml: as JSON, whether its VM exists and runs, whether its start commands run, the ports it serves and whether each reaches the Mac, and what project_up would change.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpProject) (*mcp.CallToolResult, any, error) {
			file, err := projectFileIn(in.Dir)
			if err != nil {
				return nil, nil, err
			}
			return cli(ctx, "ps", "--json", "--file="+file)
		})
	mcp.AddTool(s, tool("project_plan", "Plan a project's VM", "For a checkout with a .limabean.yaml: what project_up would do, changing nothing. For a new VM, its packs, size, network, secrets, variables, setup and start commands; for an existing one, what differs from the file. Show this to the user before calling project_up. Network changes that would widen what the VM reaches are listed as commands only the user can run.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpProject) (*mcp.CallToolResult, any, error) {
			file, err := projectFileIn(in.Dir)
			if err != nil {
				return nil, nil, err
			}
			return cli(ctx, "up", "--preview", "--file="+file)
		})
	mcp.AddTool(s, ask(tool("project_up", "Bring up a project's VM", "For a checkout with a .limabean.yaml: create its VM (cloning the repository, running its setup) or start it and apply the file's changes, then run its start commands in the background. The user is asked first; call project_plan and show them its output before this. It never widens the VM's network, and refuses changes that need a rebuild unless the server allows destructive tools.", false, &no)),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpProject) (*mcp.CallToolResult, any, error) {
			file, err := projectFileIn(in.Dir)
			if err != nil {
				return nil, nil, err
			}
			return cli(ctx, "up", "--yes", "--file="+file)
		})
	mcp.AddTool(s, tool("project_down", "Stop a project's VM", "For a checkout with a .limabean.yaml: stop its VM, keeping everything in it; project_up starts it again.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpProject) (*mcp.CallToolResult, any, error) {
			file, err := projectFileIn(in.Dir)
			if err != nil {
				return nil, nil, err
			}
			return cli(ctx, "down", "--file="+file)
		})
	mcp.AddTool(s, tool("project_logs", "Read a project's logs", "For a checkout with a .limabean.yaml: the end of its VM's provisioning log, or of its setup or start output. Read it when project_up fails, or to see what a dev server started by start printed.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpProjectLogs) (*mcp.CallToolResult, any, error) {
			file, err := projectFileIn(in.Dir)
			if err != nil {
				return nil, nil, err
			}
			args := []string{"logs", "--file=" + file}
			switch in.Log {
			case "", "provision":
			case "setup", "start":
				args = append(args, "--"+in.Log)
			default:
				return nil, nil, fmt.Errorf("log %q: provision, setup or start", in.Log)
			}
			lines := 100
			if in.Lines > 0 {
				lines = min(in.Lines, 2000)
			}
			return cli(ctx, append(args, "-n", strconv.Itoa(lines))...)
		})

	mcp.AddTool(s, tool("list_vms", "List VMs", "List limabean VMs as JSON: name, status (Running or Stopped), environment, size, number of secrets and SSH host.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return cli(ctx, "ls", "--json")
		})
	mcp.AddTool(s, tool("create_vm", "Create a VM", "Create and boot a new Ubuntu VM with core shell tools, Docker, Python and the user's shared secrets. Pass with to choose the packs of tools the task needs (go, node, python, rust, kubernetes, claude, …); without it the VM gets only the user's PACKS setting, often nothing more. Takes as long as a boot when an image of that kind exists, and a few minutes otherwise. Pass allow (and locked) to make a sandbox that can only reach what the task needs, and ttl to have it deleted later.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpCreate) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name, in.Env); err != nil {
				return nil, nil, err
			}
			if in.CPUs < 0 || in.Memory < 0 || in.Disk < 0 {
				return nil, nil, errors.New("cpus, memory and disk must be positive")
			}
			args := []string{"new"}
			if in.CPUs > 0 {
				args = append(args, "--cpus", strconv.Itoa(in.CPUs))
			}
			if in.Memory > 0 {
				args = append(args, "--memory", strconv.Itoa(in.Memory))
			}
			if in.Disk > 0 {
				args = append(args, "--disk", strconv.Itoa(in.Disk))
			}
			// Validated here, and passed as --allow=<value> so nothing reads
			// as a flag of its own.
			if _, err := expandAllow(in.Allow); err != nil {
				return nil, nil, err
			}
			for _, d := range in.Allow {
				args = append(args, "--allow="+d)
			}
			for _, p := range in.With {
				if err := checkNames(p); err != nil {
					return nil, nil, err
				}
				args = append(args, "--with="+p)
			}
			if in.Locked {
				args = append(args, "--locked")
			}
			if in.TTL != "" {
				if _, err := time.ParseDuration(in.TTL); err != nil {
					return nil, nil, fmt.Errorf("ttl %q: use a duration such as 2h or 90m", in.TTL)
				}
				args = append(args, "--ttl="+in.TTL)
			}
			args = append(args, "--", in.Name)
			if in.Env != "" {
				args = append(args, in.Env)
			}
			return cli(ctx, args...)
		})
	mcp.AddTool(s, tool("clone_vm", "Copy a VM", "Make a new VM that starts as an exact copy of another: system, packs, ~/code (with uncommitted work), secrets and network policy. Takes seconds; the source VM is shut down briefly and started again. Use it to try risky changes on a copy, or to give several attempts the same starting point.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpCloneVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name, in.NewName); err != nil {
				return nil, nil, err
			}
			if in.NewName == "" {
				return nil, nil, errors.New("new_name is required")
			}
			return cli(ctx, "clone", "--", in.Name, in.NewName)
		})
	mcp.AddTool(s, tool("start_vm", "Start a VM", "Start a stopped VM.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "start", "--", in.Name)
		})
	mcp.AddTool(s, tool("stop_vm", "Stop a VM", "Shut a VM down. Its disks are kept; start_vm brings it back.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "stop", "--", in.Name)
		})
	mcp.AddTool(s, tool("run_in_vm", "Run a command in a VM", "Run a shell command inside a running VM (never on the Mac) and return its exit code and output. Not interactive: commands that wait for input will time out.", false, &yes),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpRun) (*mcp.CallToolResult, any, error) {
			timeout := mcpDefaultTimeout
			if in.Timeout > 0 {
				timeout = min(time.Duration(in.Timeout)*time.Second, mcpMaxTimeout)
			}
			return a.mcpRun(ctx, in.Name, in.Command, timeout)
		})
	mcp.AddTool(s, tool("clone_repo", "Clone a repository", "Clone a git repository into a running VM with ghq, under ~/code/<host>/<owner>/<repo>.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpClone) (*mcp.CallToolResult, any, error) {
			if in.Repo == "" || strings.HasPrefix(in.Repo, "-") {
				return nil, nil, fmt.Errorf("%q isn't a repository", in.Repo)
			}
			return a.mcpRun(ctx, in.Name, "ghq get "+secrets.ShellQuote(in.Repo), mcpDefaultTimeout)
		})
	mcp.AddTool(s, tool("list_ports", "List forwarded ports", "List the TCP ports programs in a running VM listen on, as JSON, and whether each reaches http://localhost:<port> on the Mac.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "ports", "--json", "--", in.Name)
		})
	mcp.AddTool(s, tool("network_log", "Show a VM's DNS lookups", "List the names a running VM with an allowlist looked up since it started, as JSON, and whether each was allowed. When something fails to reach a host in a sandbox, the blocked name is here; the user can allow it with limabean network allow.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "network", "log", "--json", "--", in.Name)
		})
	mcp.AddTool(s, tool("snapshot_save", "Save a snapshot", "Save a VM's system and ~/code as a snapshot to roll back to later. The VM is shut down for a few seconds and started again.", false, &no),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpSnapshot) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			args := []string{"snapshot", "save", "--", in.Name}
			if in.Tag != "" {
				if err := validSnapshotTag(in.Tag); err != nil {
					return nil, nil, err
				}
				args = append(args, in.Tag)
			}
			return cli(ctx, args...)
		})
	mcp.AddTool(s, tool("snapshot_list", "List snapshots", "List a VM's snapshots as JSON, oldest first.", true, nil),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "snapshot", "ls", "--json", "--", in.Name)
		})

	if !allowDestructive {
		return s
	}
	mcp.AddTool(s, ask(tool("snapshot_restore", "Restore a snapshot", "Put a VM's system and ~/code back the way a snapshot saved them. Everything changed since is lost, pushed or not.", false, &yes)),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpSnapshot) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			args := []string{"snapshot", "restore", "--yes", "--", in.Name}
			if in.Tag != "" {
				if err := validSnapshotTag(in.Tag); err != nil {
					return nil, nil, err
				}
				args = append(args, in.Tag)
			}
			return cli(ctx, args...)
		})
	mcp.AddTool(s, ask(tool("delete_vm", "Delete a VM", "Delete a VM, its ~/code disk and its snapshots. Refuses while a repository in ~/code has uncommitted, stashed or unpushed work.", false, &yes)),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "rm", "--", in.Name)
		})
	mcp.AddTool(s, ask(tool("rebuild_vm", "Rebuild a VM", "Recreate a VM from the current templates. ~/code is kept when it's on its own disk; snapshots are deleted.", false, &yes)),
		func(ctx context.Context, _ *mcp.CallToolRequest, in mcpVM) (*mcp.CallToolResult, any, error) {
			if err := checkNames(in.Name); err != nil {
				return nil, nil, err
			}
			return cli(ctx, "rebuild", "--", in.Name)
		})
	return s
}

// projectFileIn finds the project file directly in dir, an absolute path
// to a checkout. Only that file is read: no parent directories, and no
// other path, so a tool call can't make limabean read anything else.
func projectFileIn(dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("dir %q: give the checkout's absolute path", dir)
	}
	dir = filepath.Clean(dir)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("%s isn't a directory", dir)
	}
	var found []string
	for _, name := range project.FileNames {
		if fi, err := os.Lstat(filepath.Join(dir, name)); err == nil && fi.Mode().IsRegular() {
			found = append(found, filepath.Join(dir, name))
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("%s has no .limabean.yaml; create_vm makes a VM without one", dir)
	case 2:
		return "", fmt.Errorf("%s has both .limabean.yaml and .limabean.yml", dir)
	}
	return found[0], nil
}

// checkNames validates VM and environment names before they reach the
// command line, where "--force" would otherwise be taken as a flag.
func checkNames(names ...string) error {
	for i, n := range names {
		if n == "" && i > 0 {
			continue // optional
		}
		if err := validVMName(n); err != nil {
			return fmt.Errorf("%q: %v", n, err)
		}
	}
	return nil
}

// runCLI runs a limabean command in-process with plain output captured,
// never prompting, and returns what it printed.
func (a *App) runCLI(ctx context.Context, args ...string) (string, error) {
	var out bytes.Buffer
	sub := *a
	sub.V = config.New()
	sub.Cfg = nil
	sub.UI = ui.NewPlain(&out)
	// Nothing may take over the terminal: stdio carries the protocol.
	sub.execSSH = func(lima.Instance, ...string) error { return errors.New("not available over MCP") }
	// What would attach to a terminal (a project's setup) runs without one,
	// its output going into the reply.
	sub.attachSSH = func(inst lima.Instance, command ...string) error {
		got, err := a.Lima.Runner.Output(ctx, nil, true, "ssh", lima.SSHArgs(inst, true, command...)...)
		out.WriteString(tail(got) + "\n")
		return err
	}
	root := NewRootCmd(&sub)
	if a.configPath != "" {
		args = append([]string{"--config", a.configPath}, args...)
	}
	root.SetArgs(append([]string{"--plain"}, args...))
	root.SetIn(strings.NewReader(""))
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.ExecuteContext(ctx)
	return strings.TrimSpace(out.String()), err
}

// mcpRun runs a shell command in a running VM, returning its exit code and
// output (stdout and stderr together, the tail if it's long).
func (a *App) mcpRun(ctx context.Context, name, command string, timeout time.Duration) (*mcp.CallToolResult, any, error) {
	if err := checkNames(name); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(command) == "" {
		return nil, nil, errors.New("command is empty")
	}
	inst, err := a.vm(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	if !inst.Running() {
		return nil, nil, fmt.Errorf("%s is stopped; start it with start_vm", name)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := a.Lima.Runner.Output(ctx, nil, true, "ssh", lima.SSHArgs(inst, true, command)...)
	code := 0
	var exit *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return nil, nil, fmt.Errorf("timed out after %s\n%s", timeout, tail(out))
	case errors.As(err, &exit):
		code = exit.ExitCode()
	case err != nil:
		return nil, nil, fmt.Errorf("%v\n%s", err, tail(out))
	}
	res := text(fmt.Sprintf("exit code %d\n%s", code, tail(out)))
	res.IsError = code != 0
	return res, nil, nil
}

// tail keeps the end of long output, where errors usually are.
func tail(out []byte) string {
	if len(out) <= mcpMaxOutput {
		return string(out)
	}
	return fmt.Sprintf("[first %d bytes cut]\n%s", len(out)-mcpMaxOutput, out[len(out)-mcpMaxOutput:])
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
