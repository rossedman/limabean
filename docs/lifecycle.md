# Rebuilds, snapshots and autostop

VMs are meant to be thrown away and recreated. limabean keeps that cheap:
your repositories survive a rebuild, a snapshot rolls a VM back in seconds,
and VMs nobody is using stop by themselves.

## Rebuilds keep `~/code`

`limabean rebuild dev` deletes the VM and creates it again from the current
templates: this is how template, tool and limabean changes reach an existing
VM. Its environment, secrets, network policy and Claude Desktop entry are
kept.

`~/code` lives on a disk of its own, so a rebuild keeps your repositories.
The disk is a Lima disk (`code-<hash>`), 50 GiB by default (`CODE_DISK`) and
allocated as it fills; `df -h ~/code` in the VM shows it. Rebuild shuts the VM
down cleanly first, so everything written reaches the disk. `limabean rm`
deletes the disk with the VM.

## Unsaved work is protected

`rm` checks every git repository under `~/code` first. If any has
uncommitted changes, stashes or commits not pushed to a remote, it stops and
lists them; in a terminal it then asks before deleting. `--force` skips both.

`rebuild` runs the same check for a VM without a code disk: one created with
`CODE_DISK` empty, or before limabean had code disks, whose `~/code` a
rebuild would erase.

## Snapshots

```bash
limabean snapshot save dev before-refactor
limabean snapshot ls dev
limabean snapshot restore dev before-refactor
limabean snapshot rm dev before-refactor
```

A snapshot saves the VM's system disk and its `~/code` disk. Restoring puts
both back exactly as they were, which makes it safe to let Claude try
something drastic. Leave out the name to save under the date and time, or to
restore the latest. In a terminal, restore asks first; `--force` doesn't.
Claude can save snapshots [through MCP](mcp.md), and restore them if you
allow it.

Lima's own `limactl snapshot` doesn't support the `vz` VMs limabean uses, so
limabean copies the disk files itself as APFS clones (`cp -c`). That takes
seconds and only uses space for blocks that change afterwards, but the VM is
shut down while it happens; limabean starts it again if it was running.
Snapshots live in the VM's Lima directory, so `rm` and `rebuild` delete them.

## Idle VMs stop by themselves

```bash
limabean autostop on              # stop VMs nobody has used for 2 hours
limabean autostop on --after 4h   # a longer grace period
limabean autostop status
limabean autostop off
```

`autostop on` installs a launchd agent that runs `limabean stop --idle 2h`
every 10 minutes, so a VM you've forgotten about stops holding memory and
battery. Its disks are kept, and `limabean ssh`, `code` or `start` brings it
back. A VM counts as in use while:

- a terminal in it (a shell, tmux, `limabean ssh`) had input or output
  within the limit. A detached tmux pane that is printing counts; one that
  sits silent doesn't.
- Claude Desktop, VS Code or Cursor is connected to it.
- its 5-minute load average is 1 or more, as during a build or with busy
  containers.
- it started less than the limit ago.

Quiet background work, such as idle containers or a long command run over
`ssh` without a terminal, doesn't keep a VM running. Run
`limabean stop --idle 2h --dry-run` to see what would stop and why the rest
are kept. The agent's output is in `~/Library/Logs/limabean-autostop.log`.
