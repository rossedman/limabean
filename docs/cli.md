# The `limabean` command

`limabean` is a single Go binary that drives `limactl`. Everything it does is
plain Lima, so `limactl` keeps working alongside it. Every command has full
help: `limabean <command> --help` explains it and shows examples.

| Command | What it does |
|---|---|
| `limabean init` | Walks through first-time setup (git config directory, code directory, the SSH `Include`, an optional GitHub token), then runs `doctor` |
| `limabean doctor` | Checks Lima, the SSH `Include` and its position, agent keys, git config, Claude Desktop, the GitHub token and every secret reference |
| `limabean bump [file…]` | Updates the pinned versions in mise TOML files (default `templates/mise/*.toml`) within each major, with `mise latest` on the Mac or `--vm` |
| `limabean new [name] [env]` | Creates a VM. In a terminal, a form asks for missing values and which secrets the VM gets; `--allow` and `--locked` set its [network policy](#network-allowlists) |
| `limabean ls` | Lists VMs with status, environment, size, number of secrets and SSH host; `--json` prints them for scripts |
| `limabean ssh [name] [cmd…]` | Opens a shell or runs a command, starting a stopped VM first |
| `limabean get [name] <repo>` | Clones a repository into `~/code` with ghq; flags after the VM name (like `-p`) go to ghq |
| `limabean code [name] [repo]` | Opens the VM's `~/code`, or one clone in it, in VS Code, Cursor or another editor with VS Code's `--remote` flag, over SSH; `--ide` picks the editor |
| `limabean ports [name]` | Lists the TCP ports programs in the VM listen on and whether each reaches your Mac's localhost; `--json` for scripts |
| `limabean open [name] [port]` | Opens `http://localhost:<port>` in your browser; without a port, the VM's only forwarded one, or a picker |
| `limabean start [name]` | Starts a stopped VM and adds it back to Claude Desktop |
| `limabean stop [name]` | Stops a VM and removes it from Claude Desktop until it starts again; `--idle 2h` stops only VMs [nobody is using](#idle-vms-are-stopped-automatically) |
| `limabean rebuild [name]` | Recreates the VM from the current templates, keeping its environment, secrets and, on its own disk, `~/code` |
| `limabean rm [name]` | Deletes a VM, its `~/code` disk, its Claude Desktop entry and its own secrets |
| `limabean snapshot save\|ls\|restore\|rm` | Saves a VM's disks, `~/code` included, and [rolls back](#snapshots) to them |
| `limabean autostop on\|off\|status` | Runs `stop --idle` every 10 minutes in the background, through a launchd agent |
| `limabean secrets ls\|set\|rm\|edit\|sync` | Manages the environment variables and secrets VMs get ([details](secrets.md)) |
| `limabean github login\|status\|sync\|logout` | Manages the GitHub token VMs use ([details](github.md)) |
| `limabean network ls\|allow\|rm\|open\|lock\|unlock` | Manages a VM's [network allowlist and lock](#network-allowlists) |
| `limabean claude sync` | Registers running VMs in Claude Desktop and removes stopped or deleted ones |
| `limabean mcp` | Runs an MCP server so Claude can manage VMs ([details](claude-desktop.md#let-claude-manage-vms)) |
| `limabean completion <shell>` | Prints a tab-completion script for bash, zsh, fish or PowerShell |
| `limabean version` | Prints the version |

Leave out a VM name and you pick one from a list.

## Interactive and plain mode

In a terminal, limabean shows its interface:
- **Spinners** for slow steps. Lima's output is shown only if a step fails.
- **Pickers and forms** for anything you didn't pass on the command line.
- **Confirmations** before `rm` and `rebuild`.
- **Tables** for `ls` and `secrets ls`.

When output isn't a terminal (pipes, scripts, CI), or with `--plain` or
`LIMABEAN_PLAIN=1`, it prints plain text and never prompts: missing names
are errors, and questions take their defaults.

## Unsaved work is protected

`~/code` is on a disk of its own (`CODE_DISK`, 50 GiB by default and
allocated as it fills), so `rebuild` erases the system and keeps your
repositories. It shuts the VM down cleanly first, so everything written
reaches the disk. `rm` deletes the disk with the VM.

`rm`, and `rebuild` for a VM without a code disk (one created with
`CODE_DISK` empty, or before limabean had it), first check every git
repository under `~/code`. If any has uncommitted changes, stashes or
commits not pushed to a remote, they stop and list what they found. Then,
in a terminal, they ask before deleting. `--force` skips both.

## Network allowlists

A VM can be limited to the domains its work needs, for instance before
handing it to an agent:

```bash
limabean new agent --allow github --allow python --locked
limabean network allow agent registry.npmjs.org
limabean network ls agent
```

Everything else is blocked, for the VM and its containers, and names that
aren't allowed don't resolve, so data can't leave through DNS either. A
domain allows its subdomains. Presets: `claude`, `docker`, `github`, `go`,
`npm`, `python`, `ubuntu` (`limabean network --help` lists their domains).
`ntp.ubuntu.com` is always allowed, for the clock.

**Lock it, or it's only a guardrail.** A Lima VM's user has passwordless
`sudo`, and Docker's socket is root-equivalent, so anything in an unlocked VM
can turn the allowlist off. `--locked` (or `limabean network lock`) takes both
away from the user; the lock is re-applied on every boot. Lock before giving
the VM to an agent: a root process started earlier keeps running.

The policy lives on the Mac in `~/.config/limabean/network/<vm>/` and is
mounted read-only into the VM, where `limabean-network` applies it with
dnsmasq and nftables. Changes apply to a running VM at once. The first boot
provisions with the network open; the policy applies when `new` or `rebuild`
finishes. VMs created before this need a rebuild to use it.

## Idle VMs are stopped automatically

`limabean autostop on` installs a launchd agent that runs
`limabean stop --idle 2h` every 10 minutes (`--after` changes the limit), so a
VM you've forgotten about stops holding memory and battery. Its disk is kept,
and `ssh`, `code` or `start` brings it back. A VM counts as in use while:

- a terminal in it (a shell, tmux, `limabean ssh`) had input or output
  within the limit. A detached tmux pane that is printing counts; one that
  sits silent doesn't.
- Claude Desktop, VS Code or Cursor is connected to it.
- its 5-minute load average is 1 or more, as during a build or with busy
  containers.
- it started less than the limit ago.

Quiet background work, such as idle containers or a long command run over
`ssh` without a terminal, doesn't keep a VM running. Run
`limabean stop --idle 2h --dry-run` to see what would stop and why the
rest are kept. The agent's output is in `~/Library/Logs/limabean-autostop.log`.

## Snapshots

`limabean snapshot save dev before-refactor` saves the VM's system disk and,
when `~/code` is on its own disk, that too. `limabean snapshot restore dev
before-refactor` puts both back exactly as they were, which makes it safe to
let Claude try something drastic. Leave out the name to save under the date
and time, or to restore the latest.

Lima's own `limactl snapshot` doesn't support the `vz` VMs limabean uses,
so limabean copies the disk files itself as APFS clones (`cp -c`). That
takes seconds and only uses space for blocks that change afterwards, but the
VM has to be shut down while it happens; limabean starts it again if it was
running. Snapshots live in the VM's Lima directory, so `rm` and `rebuild`
delete them.

## Settings

`~/.config/limabean/config` is a small `KEY="value"` file written by `init`.
Each setting can also come from a `LIMABEAN_*` environment variable, or a
flag where one exists, which take priority:

```bash
GIT_CONFIG_DIR="~/.config/git"   # host directory mounted read-only into VMs
CODE_DIR="~/code"                # where ghq clones inside VMs
CODE_DISK="50"                   # GiB of the disk CODE_DIR lives on, kept by rebuild; empty: none
CPUS=""                          # empty: template default (4); flag: new --cpus
MEMORY=""                        # GiB; empty: template default (8); flag: new --memory
CLAUDE_DESKTOP="yes"             # register VMs as SSH connections in Claude Desktop
AWS_DIR="~/.aws"                 # mounted read-only at ~/.aws in VMs if it exists; empty: never
IDE="code"                       # editor limabean code opens: code, cursor, …; flag: code --ide
```

Changes apply to new VMs, and to existing ones after `limabean rebuild`.
IDE is the exception: it only affects `limabean code`, so it applies at once.

Global flags:

| Flag | |
|---|---|
| `--config <file>` | Use another config file |
| `--plain` | Plain text, no prompts (`LIMABEAN_PLAIN=1`) |
| `--templates <dir>` | Read templates from a directory instead of the built-in copy (`LIMABEAN_TEMPLATES`); see [Customizing](customizing.md) |
