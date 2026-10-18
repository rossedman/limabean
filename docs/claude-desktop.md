# Claude Desktop SSH connections

`limabean new` registers every VM in the Claude desktop app as an SSH
connection, and `limabean rm` removes it. Only running VMs are listed:
Claude Desktop shows an error for a connection it can't reach, so
`limabean stop` removes the entry and `limabean start` (or `ssh`/`get`, which
start a stopped VM) adds it back. In the prompt box's environment
dropdown, pick **SSH → `<name> (limabean)`** and open a directory under
`~/code`. Commands Claude runs use the VM's tools and can't touch files on your
Mac.

On first connect, Claude Desktop installs its own copy of Claude Code in the
VM under `~/.claude/remote/`, which takes about 1 GB.

## Guardrails

Every VM turns on the [guardrails](https://github.com/rossedman/guardrails)
plugin for Claude Code, so a session in a VM is held to the same standards
as one on your Mac: files are formatted and linted as Claude edits them, and
Claude can't finish a turn until the code it touched builds, passes its
tests and meets the coverage bar. The VM's packs supply the tools it runs
(gofumpt, goimports, golangci-lint, ruff, yamllint, shellcheck, shfmt), and
it downloads any others on first use.

limabean adds two entries to the VM's `~/.claude/settings.json`: the
`guardrails` marketplace (`rossedman/guardrails` on GitHub) and
`guardrails@guardrails` in `enabledPlugins`. Claude Code fetches the plugin
when a session starts. That covers both Claude Desktop's copy and the
[`claude` pack](packs.md#claude-code-in-a-vm)'s.

- To turn it off in a VM, set `"guardrails@guardrails": false` in its
  `~/.claude/settings.json`. limabean only adds entries that are missing,
  so your choice stays.
- A VM with a [network allowlist](network.md) needs the `github` preset for
  Claude Code to fetch the plugin and its tools.

## Let Claude manage VMs too

An SSH connection puts a Claude session inside one VM. To let Claude, running
on your Mac, create VMs, run commands in them and snapshot them, add
limabean's MCP server: see [Claude and MCP](mcp.md).

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
