# Claude Desktop

`limabean new` registers every VM in the Claude desktop app as an SSH
connection, and `limabean rm` removes it. Only running VMs are listed:
Claude Desktop shows an error for a connection it can't reach, so
`limabean stop` removes the entry and `limabean start` (or `ssh`/`get`, which
start a stopped VM) adds it back. In the prompt box's environment
dropdown, pick **SSH → `<name> (limabean)`** and open a directory under
`~/code`. Commands Claude runs use the VM's tools and can't touch files on your
Mac.

Nothing Claude-specific is provisioned. On first connect, Claude Desktop
installs its own copy of Claude Code in the VM under `~/.claude/remote/`, which
takes about 1 GB.

## Let Claude manage VMs

The SSH connection puts Claude inside one VM. `limabean mcp` goes further:
it's an MCP server that lets Claude, in Claude Code or Claude Desktop on
your Mac, manage VMs itself. It can make a scratch VM for a task, clone a
repository into it, run commands, checkpoint it with a snapshot before
something risky, and stop it when it's done.

```bash
# Claude Code
claude mcp add limabean -- limabean mcp
```

For Claude Desktop, add it to `claude_desktop_config.json`, using the full
path from `which limabean`:

```json
{ "mcpServers": { "limabean": { "command": "/opt/homebrew/bin/limabean", "args": ["mcp"] } } }
```

| Tool | Does |
|---|---|
| `list_vms` | Lists VMs and their status, as JSON |
| `create_vm` | Creates a VM (name, environment, CPUs, memory) with your shared secrets; `allow` and `locked` make it a [sandbox](cli.md#network-allowlists) that can only reach what the task needs |
| `start_vm`, `stop_vm` | Starts or stops a VM |
| `run_in_vm` | Runs a shell command in a running VM and returns its exit code and output |
| `clone_repo` | Clones a repository into `~/code` with ghq |
| `list_ports` | Lists ports the VM forwards to your Mac |
| `snapshot_save`, `snapshot_list` | Saves and lists snapshots |
| `snapshot_restore`, `delete_vm`, `rebuild_vm` | Only with `limabean mcp --allow-destructive` |

What it can't do:
- **Run anything on your Mac.** Commands run inside VMs only.
- **Read or add secrets.** Values never reach Claude, and new VMs get only
  your shared secrets: adding one could run a `cmd:` reference on the Mac.
- **Delete, rebuild or restore without your say-so.** Those tools need
  `--allow-destructive`, are marked destructive so the client asks you, and
  `delete_vm` still stops at unsaved work.

A command in a VM runs as you there, with that VM's secrets and GitHub
token, so give VMs only the secrets an agent should be able to use.

## How it works

Claude Desktop reads connections from the `sshConfigs` list in
`~/.claude/settings.json` ([docs](https://code.claude.com/docs/en/desktop)).
limabean adds one entry per VM, with `sshHost` set to the VM's `lima-<name>`
alias:

```json
{ "id": "limabean-dev", "name": "dev (limabean)", "sshHost": "lima-dev" }
```

- Only entries whose id starts with `limabean-` are touched. Your other
  settings are left alone.
- The file isn't changed if it isn't valid JSON. It is written in place, so
  permissions and symlinks survive.
- A VM deleted with `limactl delete`, or stopped or started with `limactl`
  or by a Mac restart, has its entry fixed the next time you run
  `limabean ls` or `limabean new`. `limabean rm <name>` cleans it up too.
- `limabean claude sync` reconciles everything at once.
- Restart Claude Desktop if a new VM doesn't appear.
- To opt out, set `CLAUDE_DESKTOP="no"` in `~/.config/limabean/config`.
