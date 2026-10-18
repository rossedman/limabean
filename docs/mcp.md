# Claude and MCP

limabean includes an MCP server, so Claude can manage your VMs itself. From
Claude Code or Claude Desktop on your Mac, Claude can create a VM for a task,
clone a repository into it, run the build and tests, snapshot it before
something risky, and stop it when it's done. Nothing it runs touches your Mac.

```bash
claude mcp add --scope user limabean -- limabean mcp
```

That's the whole setup for Claude Code. Then ask for something like:

> Make a VM called scratch, clone rossedman/limabean into it, and run the tests.

!!! tip "MCP or an SSH connection?"
    They work together. **MCP** lets Claude, running on your Mac, manage VMs
    and run commands in them. An **SSH connection**, which `limabean up`
    adds to Claude Desktop for every VM, puts a whole Claude session inside
    one VM. See [Claude Desktop](claude-desktop.md) for that.

## Set it up

### Claude Code

```bash
claude mcp add --scope user limabean -- limabean mcp
```

`--scope user` makes it available in all your projects. Without it, Claude
Code uses `local` scope: just you, and only in the project where you ran the
command. Check it with `claude mcp list`, or `/mcp` inside a session, which
also lists the tools. To remove it: `claude mcp remove limabean`.

Arguments after `--` go to limabean, so this also offers the
[destructive tools](#tools-that-destroy-work):

```bash
claude mcp add --scope user limabean -- limabean mcp --allow-destructive
```

### Claude Desktop

1. Open **Claude → Settings… → Developer → Edit Config**. That opens
   `~/Library/Application Support/Claude/claude_desktop_config.json`,
   creating it if needed.
2. Add limabean under `mcpServers`:

    ```json
    {
      "mcpServers": {
        "limabean": {
          "command": "limabean",
          "args": ["mcp"]
        }
      }
    }
    ```

3. Quit Claude Desktop completely and open it again.

If the server doesn't show up, use the full path from `which limabean` as the
`command`, for example `/opt/homebrew/bin/limabean`: apps started from the
Dock don't always see your shell's `PATH`. Claude Desktop's logs are in
`~/Library/Logs/Claude`; `mcp-server-limabean.log` has limabean's own output.

Claude Code can copy servers from Claude Desktop's config with
`claude mcp add-from-claude-desktop`.

## What to ask for

Claude picks the tools; you describe the task.

- *"Create a VM called api with 8 GB of memory, clone acme/api into it and get
  the test suite passing."*
- *"What's listening in dev? Which of those can I open on my Mac?"*
- *"Snapshot dev, then try upgrading the project to Go 1.27. If the tests
  fail, tell me what broke."*
- *"Make a locked sandbox that can only reach GitHub and PyPI, clone
  acme/scraper into it and run it."*
- *"Clone dev three times and try a different fix for the flaky test in each.
  Tell me which one worked."*
- *"Make a scratch VM that deletes itself in two hours and benchmark the
  three JSON libraries in it."*
- *"Stop every VM I'm not using."*

Repositories go to `~/code/<host>/<owner>/<repo>` in the VM, so after
`clone_repo` Claude runs commands with `cd ~/code/github.com/acme/api && …`.

## Sandboxes for a task

`create_vm` takes `allow` and `locked`, so Claude can make itself a VM that
can only reach what the task needs:

> Create a VM called triage that can only reach github and python, locked.
> Clone acme/scraper, install its dependencies and run the test suite.

The VM gets an [allowlist](network.md): every other domain is blocked, for
the VM and its containers. **Locked** takes sudo and Docker away from the
VM's user, so nothing in it, Claude included, can lift the allowlist. An agent
can narrow a new VM's network this way; no tool can widen one. Only you can,
with `limabean network`.

Pair it with snapshots: *"snapshot triage before you change anything"*, then
restore from the command line (`limabean snapshot restore triage`) if
something goes wrong.

When something in the sandbox can't reach a host, Claude can see which name
was blocked with `network_log`, and tell you; adding it is up to you
(`limabean network allow triage <domain>`).

## Claude Code inside a VM

MCP lets Claude on your Mac drive VMs. To run an agent *inside* one
instead, give the VM the [`claude` pack](packs.md#claude-code-in-a-vm) and a
login secret, and lock it down: the agent works with the VM's tools and
network, never your Mac's. Combine it with a [project file](projects.md)
(`packs: [claude]`) or with
[`limabean run --rm`](lifecycle.md#throwaway-vms) for a VM that's gone when
the task is done.

## Projects

When a checkout has a [`.limabean.yaml`](projects.md), Claude can use it the
way you use `limabean up`: *"bring up the VM for ~/src/shop and get its
tests passing"*. The project tools take the checkout's absolute path:

1. `project_plan` shows what `up` would do: a new VM's packs, size,
   network, environment, setup and start commands, or what differs from an
   existing one. Claude should show you this first.
2. `project_up` does it. Claude Code asks you before **every** call, so you
   approve with the plan in front of you. Changed `start` commands count as
   approved when you say yes.
3. `project_status`, `project_logs` and `project_down` check on it, read its
   provisioning, setup or start output, and stop it.

The same rules hold as in your terminal, plus a few for MCP:

- A file never widens a VM's network; those changes come back as
  `limabean network` commands for you to run.
- A change that rebuilds the VM (new packs or environment template) is
  refused unless the server runs with `--allow-destructive`.
- Only the directory's own `.limabean.yaml` is read: not a parent
  directory's, not a symlink, not any other path, so a tool call can't make
  limabean read something else on your Mac.
- The checkout's origin is read from `.git/config` as a file; git doesn't
  run on the Mac.
- Setup runs without a terminal, and its output comes back to Claude.

## Tools

| Tool | Does |
|---|---|
| `project_plan` | What `up` would create or change for a checkout's `.limabean.yaml`, changing nothing |
| `project_up` | Creates or updates the checkout's VM and runs its start commands; Claude Code asks you first ([projects](#projects)) |
| `project_status` | The checkout's VM as JSON: status, start, ports, pending changes |
| `project_logs` | The end of its provisioning, setup or start log |
| `project_down` | Stops the checkout's VM |
| `list_vms` | Lists VMs: status, environment, size, secrets, network |
| `create_vm` | Creates a VM: name, and optionally environment, CPUs, memory, `with` (the [packs](packs.md) the task needs; without it, only core and the `PACKS` setting's), `allow`, `locked`, and `ttl` to have it deleted later. It gets your shared secrets |
| `clone_vm` | Copies a VM in seconds, `~/code` and all ([clones](lifecycle.md#clones)) |
| `start_vm`, `stop_vm` | Starts or stops a VM |
| `run_in_vm` | Runs a shell command in a running VM and returns the exit code and output |
| `clone_repo` | Clones a repository into `~/code` with ghq |
| `list_ports` | Lists the ports programs in a VM listen on, and which reach `http://localhost:<port>` on the Mac |
| `network_log` | Lists the names a sandboxed VM looked up, and which its allowlist [blocked](network.md#whats-allowed) |
| `snapshot_save`, `snapshot_list` | Saves and lists [snapshots](lifecycle.md#snapshots) |
| `snapshot_restore`, `delete_vm`, `rebuild_vm` | Only with `--allow-destructive` |

`run_in_vm` runs as you, in your home directory, through your login shell. It
isn't interactive: a command that waits for input times out. The default
timeout is 10 minutes, and Claude can ask for up to an hour. Long output is
trimmed to its last 100 KiB, where errors usually are.

### Tools that destroy work

`snapshot_restore`, `delete_vm` and `rebuild_vm` only exist if you start the
server with `limabean mcp --allow-destructive`. Even then:

- Claude Code asks you before **every** call to them, whatever its
  permission mode, with no "don't ask again".
- `delete_vm` refuses while a repository in `~/code` has uncommitted, stashed
  or unpushed work, just like `limabean rm`.
- They're marked destructive (`destructiveHint`) for any other MCP client
  that uses that.

## What Claude can't do

- **Run anything on your Mac.** Every command runs inside a VM, over SSH.
- **See or add secrets.** No tool returns secret values. New VMs get only
  your shared secrets: adding a secret could run a `cmd:` reference on the
  Mac.
- **Widen a VM's network.** It can create a restricted VM, but not change an
  existing VM's allowlist or lock, from a project file or otherwise.
- **Read arbitrary files.** The project tools read only a checkout's own
  `.limabean.yaml` (and its `.git/config` for the origin).
- **Delete a VM it didn't ask to be temporary.** `ttl` only applies to a VM
  as `create_vm` makes it; no tool sets one on an existing VM.
- **Pass names as flags.** VM names, environments, snapshot names and
  domains are checked before they reach limabean's command line, so a "name"
  like `--force` is refused.

A command in a VM runs as you there, with that VM's secrets and GitHub token,
and can reach whatever the VM can. Give a VM only the secrets an agent should
use, and [lock it](network.md) when it should only reach certain hosts. The
reasoning is in [design note D14](design.md).

## Troubleshooting

| Symptom | Fix |
|---|---|
| Claude Code doesn't list the tools | `claude mcp list` should show limabean. Added without `--scope user`, it only loads in the project where you added it. |
| Claude Desktop doesn't show it | Quit and reopen the app. Use the full path from `which limabean` as `command`. Check `~/Library/Logs/Claude/mcp-server-limabean.log`. |
| `limabean mcp` in a terminal prints nothing | That's right: it waits for a client on stdin. Press Ctrl-C. |
| *"dev is stopped; start it with start_vm"* | Commands only run in running VMs; ask Claude to start it. |
| *"timed out after 10m0s"* | Ask for a longer timeout (up to an hour), or run the command in the background in the VM. |
| A clone or install fails in a sandbox | The domain isn't on its allowlist. Add it from your Mac: `limabean network allow <vm> <domain>`. |
