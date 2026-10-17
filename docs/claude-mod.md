# The Claude Code mod

limabean ships a mod for Claude Code: a plugin that knows where Claude runs.

- **Inside a limabean VM**, it tells Claude which VM it's in, what the
  network allows, whether the VM is locked and which packs it has, before
  Claude's first step. It shows the same in a band above the prompt, and
  when a command fails on a host the allowlist blocks, Claude gets a note
  naming the domain and the command that allows it.
- **On your Mac**, it keeps a status line of your VMs, opens a pane with
  each VM's state, network, packs, time left and forwarded ports
  (`/limabean`), and toasts what changed behind your back: a VM autostop
  stopped, an expired `--ttl` VM, a new image.

## In a VM: nothing to install

Every VM has the mod at `/usr/local/share/limabean/claude/limabean`, and
`CLAUDE_CODE_PLUGIN_DIRS` points Claude Code at it, so `claude` run in a VM
(with the [`claude` pack](packs.md#claude-code-in-a-vm)) loads it by itself.
VMs created before the mod get it with `limabean rebuild`.

Above the prompt:

```
limabean agent · only anthropic.com, claude.ai, claude.com, github.com +2 · locked: no sudo, no Docker  Hide
```

Hide folds it into the status line (`limabean · agent · 6 domains ·
locked`); `/limabean` brings it back and prints what Claude was told.

What Claude reads, in its system prompt, for that VM:

```text
You are running inside the limabean VM "agent", an Ubuntu VM on the user's Mac. Work happens here, not on the Mac.
- Repositories live under ~/code (ghq layout: <host>/<owner>/<repo>). Push work to a remote before it matters: the VM can be rebuilt or deleted.
- Tools come from packs (mise): core, claude, node. `mise use <tool>@<version>` adds others for one project.
- The network is an allowlist: only anthropic.com, claude.ai, claude.com, github.com, githubusercontent.com, npmjs.org and their subdomains resolve. Anything else fails with "Could not resolve host". Don't try other hosts or mirrors; tell the user which domain is needed, and that they can allow it on the Mac with: limabean network allow agent <domain>
- The VM is locked: there is no sudo and no Docker for this user. Don't try either; install tools in your home directory (mise, pip --user, npm prefixes) or ask the user.
- Files on the Mac aren't reachable from here. The user moves files with `limabean cp` on the Mac.
```

The mod reads the VM's [network policy](network.md) every 30 seconds, so a
domain you allow from the Mac reaches Claude on its next request, with a
toast saying the allowlist changed.

## On your Mac: install it once

At the prompt of Claude Code in a terminal:

```
/plugin install limabean --marketplace rossedman/limabean
```

Answer `y` to add the marketplace, then pick a scope (user, for every
session). From a shell, `claude plugin install limabean --marketplace
rossedman/limabean` does the same. In the desktop app: + next to the prompt
box, then Plugins, Add marketplace with `rossedman/limabean`, then Add
plugin.

The status line reads `limabean: 2 running · 1 stopped`. `/limabean` opens
the pane:

```
● dev 2 CPU · 4 GiB · open  [ Stop ]
   packs: containers, db, go, infra, kubernetes, node, python
   :3000 node → http://localhost:3000
● agent 2 CPU · 4 GiB · 6 domain(s), locked · 1h20m left  [ Stop ]
   packs: claude, node
○ api 4 CPU · 8 GiB · open  [ Start ]

Images
  scratch + containers, db, go, infra, kubernetes, node, python · current · built 2d ago

[ Refresh ]
```

It reads `limabean ls --json` and `limabean images ls --json` every 30
seconds, and `limabean ports --json` for running VMs while the pane is
open. It changes nothing except what its Start and Stop buttons ask for.

## Settings

Under `/config`, in the plugin's section:

| Setting | Does |
|---|---|
| limabean binary | Where limabean is, if Claude Code starts without it on its PATH (as an app started from the Dock can). Empty: `limabean` on the PATH, then `/opt/homebrew/bin`, `/usr/local/bin`, `~/go/bin` and `~/.local/bin`. |
| Preview | `mac` or `vm` shows sample data instead of real VMs, to see what the mod looks like. |

## Limits

- Where an organization manages Claude Code, system prompt hooks only run
  for plugins it lists in managed settings. There the band and the note
  after a blocked host still work; the system prompt section doesn't.
- The note after a blocked host covers the usual error messages (curl,
  git, npm, pip, Go). A tool that words it differently gets no note, and
  Claude still has the system prompt section.

## Working on the mod

The mod lives in
[`templates/claude/limabean`](https://github.com/rossedman/limabean/tree/main/templates/claude/limabean),
so the limabean binary carries it into VMs. Check it with:

```bash
claude plugin validate templates/claude/limabean
claude plugin test templates/claude/limabean
```

A file added to it must also be written into VMs by `layer-base.yaml`; a Go
test (`TestClaudeModIsProvisioned`) fails until it is.
