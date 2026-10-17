# Rebuilds, clones and snapshots

VMs are meant to be thrown away and recreated. limabean keeps that cheap:
new VMs are copies of a ready-made image, your repositories survive a
rebuild, a clone or a snapshot takes seconds, throwaway VMs delete
themselves, and VMs nobody is using stop by themselves.

## Fast VMs: images

Provisioning a VM from scratch takes minutes: every Ubuntu update, Docker,
and each pack's tools. So limabean does it once per kind of VM and keeps
the result as an **image**: a provisioned VM, shut down, with no secrets,
code disk or network policy of its own. `new`, `up`, `run`, `rebuild` and
MCP's `create_vm` make an APFS clone of the image, give it the VM's own
settings, and boot it. The copy takes moments and shares disk blocks with
the image until they change, so a new VM is ready about as soon as it has
booted.

- **The first VM of a kind** (an environment and a set of packs) is
  provisioned the slow way, and its image is built in the background, so
  the next one is fast. Build one ahead of time with
  `limabean images build` (`--with rust` for other packs).
- **Images are kept fresh.** One more than a week old is still used, and
  replaced in the background, so copies never miss more than a week of
  updates. Any change to the templates or packs, a `limabean bump` or a
  pack of your own included, gives a new kind of image.
- **Size is the VM's own.** A copy gets the VM's CPUs and memory, and its
  disk grows on first boot when `DISK` or `--disk` asks for more than the
  image's 10 GiB. A VM asking for a smaller disk than its image is
  provisioned from scratch, as Lima can't shrink a disk.
- **Images are hidden** from `limabean ls`. `limabean images ls` lists them;
  `autostop run` and `limabean images prune` delete outdated ones and ones
  nobody has copied for 30 days. Each image uses a few GiB.

Background builds log to `~/Library/Logs/limabean-images.log`. To provision
every VM from scratch instead, set `IMAGES="no"` in the config.

```bash
limabean images ls
limabean images build              # the default packs, now
limabean images prune
limabean images rm --all
```

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

## Clones

```bash
limabean clone dev dev-experiment
```

A clone is a new VM that starts as an exact copy of another: its system,
packs, secrets, network policy and `~/code`, uncommitted changes included.
Like snapshots, the disks are APFS clones, so it takes seconds and the copy
only uses space for what changes in it. The VM being copied is shut down for
those seconds and started again. Snapshots aren't copied.

Clone a VM to try something risky without touching the original, or to give
several agents the same starting point and keep the one that worked:

```bash
for i in 1 2 3; do limabean clone agent agent-$i; done
```

Claude can clone VMs too, [through MCP](mcp.md#tools) (`clone_vm`).

## Throwaway VMs

```bash
limabean new spike --ttl 4h                    # deleted in four hours
limabean run --rm -- 'uname -a'                # a VM for one command
limabean run --rm --repo me/app -- make test   # a clean checkout's tests
```

`--ttl` deletes a VM once it's that old, **whatever is in it**: there's no
unsaved-work check, since that's what the TTL asked for. The
[autostop agent](#idle-vms-stop-by-themselves) deletes expired VMs every 10
minutes, so turn it on; `limabean rm --expired` does it by hand.
`limabean ls` shows how long each has left.

`limabean run` creates a VM, runs one command in it and, with `--rm`,
deletes the VM when the command ends, whether it worked or not. limabean
exits with the command's exit status, so it fits in scripts. `--repo` clones
a repository first and runs the command in it. It takes the same `--with`,
`--allow`, `--locked` and `--secret` flags as `new`:

```bash
# Claude Code on a task, in a locked sandbox, then gone
limabean run --rm --with claude --allow github --allow claude --locked \
  --repo me/app -- 'claude -p "make the tests pass" && git push'
```

Without `--rm` the VM is kept (named `run-<random>` unless you pass
`--name`), and `--ttl` can delete it later.

## Idle VMs stop by themselves

```bash
limabean autostop on              # stop VMs nobody has used for 2 hours
limabean autostop on --after 4h   # a longer grace period
limabean autostop status
limabean autostop off
```

`autostop on` installs a launchd agent that runs
`limabean autostop run --idle 2h` every 10 minutes: it deletes
[expired VMs](#throwaway-vms), and stops VMs nobody is using, so one you've
forgotten about stops holding memory and battery. An agent installed before
limabean had `--ttl` only stops idle VMs: run `limabean autostop on` again
(`autostop status` says if it's needed). Its disks are kept, and `limabean ssh`, `code` or `start` brings it
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
