# Claude Desktop

`limabean new` registers every VM in the Claude desktop app as an SSH
connection, and `limabean rm` removes it. In the prompt box's environment
dropdown, pick **SSH → `<name> (limabean)`** and open a directory under
`~/code`. Commands Claude runs use the VM's tools and can't touch files on your
Mac.

Nothing Claude-specific is provisioned. On first connect, Claude Desktop
installs its own copy of Claude Code in the VM under `~/.claude/remote/`, which
takes about 1 GB.

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
- A VM deleted with `limactl delete` loses its entry the next time you run
  `limabean ls` or `limabean new`. `limabean rm <name>` cleans it up too.
- `limabean claude sync` reconciles everything at once.
- Restart Claude Desktop if a new VM doesn't appear.
- To opt out, set `CLAUDE_DESKTOP="no"` in `~/.config/limabean/config`.
