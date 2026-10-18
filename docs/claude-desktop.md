# Claude Desktop SSH connections

`limabean up` registers every VM in the Claude desktop app as an SSH
connection, and `limabean rm` removes it. Only running VMs are listed:
Claude Desktop shows an error for a connection it can't reach, so
`limabean down` removes the entry and `limabean up` (or `ssh`/`get`, which
start a stopped VM) adds it back. In the prompt box's environment
dropdown, pick **SSH → `<name> (limabean)`** and open a directory under
`~/code`. Commands Claude runs use the VM's tools and can't touch files on your
Mac.

On first connect, Claude Desktop installs its own copy of Claude Code in the
VM under `~/.claude/remote/`, which takes about 1 GB.

## Plugins

Claude Code in a VM gets the plugins a project's
[`.limabean.yaml`](projects.md#claude-plugins) names, in that VM's
`~/.claude/settings.json`, so Claude Desktop's copy and the
[`claude` pack](packs.md#claude-code-in-a-vm)'s both use them. For example,
[guardrails](https://github.com/rossedman/guardrails), which formats and
lints Claude's edits and won't let a turn end until the tests pass:

```yaml
claude:
  marketplaces:
    - marketplace: rossedman/guardrails
      plugins: [guardrails@guardrails]
```

The packs carry the formatters, linters and language servers plugins like
these use (gofumpt, golangci-lint, ruff, shellcheck, gopls, pyright, …).

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
  `limabean ls` or `limabean up`. `limabean rm <name>` cleans it up too.
- `limabean claude sync` reconciles everything at once.
- Restart Claude Desktop if a new VM doesn't appear.
- To opt out, set `CLAUDE_DESKTOP="no"` in `~/.config/limabean/config`.
