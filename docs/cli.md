# The `limabean` command

`limabean` is a single Go binary that drives `limactl`. Everything it does is
plain Lima, so `limactl` keeps working alongside it. Every command has full
help: `limabean <command> --help` explains it and shows examples.

| Command | What it does |
|---|---|
| `limabean init` | Walks through first-time setup (git config directory, code directory, the SSH `Include`, an optional GitHub token), then runs `doctor` |
| `limabean doctor` | Checks Lima, the SSH `Include` and its position, agent keys, git config, Claude Desktop, the GitHub token and every secret reference |
| `limabean bump [file…]` | Updates the pinned versions in mise TOML files (default `templates/packs/*.toml`) within each major, with `mise latest` on the Mac or `--vm` |
| `limabean new [name] [env]` | Creates a VM. In a terminal, a form asks for missing values and which secrets the VM gets; `--with` and `--without` change its [packs](packs.md); `--allow`, `--locked` and `--learn` set its [network policy](network.md); `--ttl` [deletes it later](lifecycle.md#throwaway-vms) |
| `limabean up` | Creates or starts the VM a repository's [`.limabean.yaml`](projects.md) describes, clones the repository and runs its setup |
| `limabean run [flags] -- <cmd>` | Creates a VM, runs one command in it (in a fresh clone with `--repo`), and with `--rm` deletes it after; exits with the command's status ([details](lifecycle.md#throwaway-vms)) |
| `limabean packs` | Lists the [packs](packs.md) of tools a VM can get and which new VMs get by default; `--json` for scripts |
| `limabean ls` | Lists VMs with status, environment, size, number of secrets, network and SSH host, and when throwaway ones expire; `--json` prints them for scripts |
| `limabean ssh [name] [cmd…]` | Opens a shell or runs a command, starting a stopped VM first |
| `limabean get [name] <repo>` | Clones a repository into `~/code` with ghq; flags after the VM name (like `-p`) go to ghq |
| `limabean cp <src>… <dst>` | Copies files between the Mac and VMs; VM paths are `<vm>:<path>`, `-r` for directories |
| `limabean code [name] [repo]` | Opens the VM's `~/code`, or one clone in it, in VS Code, Cursor or another editor with VS Code's `--remote` flag, over SSH; `--ide` picks the editor |
| `limabean ports [name]` | Lists the TCP ports programs in the VM listen on and whether each reaches your Mac's localhost; `--json` for scripts |
| `limabean open [name] [port]` | Opens `http://localhost:<port>` in your browser; without a port, the VM's only forwarded one, or a picker |
| `limabean start [name]` | Starts a stopped VM and adds it back to Claude Desktop |
| `limabean stop [name]` | Stops a VM and removes it from Claude Desktop until it starts again; `--idle 2h` stops only VMs [nobody is using](lifecycle.md#idle-vms-stop-by-themselves) |
| `limabean rebuild [name]` | Recreates the VM from the current templates, keeping its environment, packs (`--with`/`--without` change them), secrets and, on its own disk, `~/code` |
| `limabean clone <name> <new>` | Copies a VM in seconds: system, packs, `~/code`, secrets and network policy ([details](lifecycle.md#clones)) |
| `limabean rm [name]` | Deletes a VM, its `~/code` disk, its Claude Desktop entry and its own secrets; `--expired` deletes every VM past its `--ttl` |
| `limabean snapshot save\|ls\|restore\|rm` | Saves a VM's disks, `~/code` included, and [rolls back](lifecycle.md#snapshots) to them |
| `limabean images ls\|build\|rm\|prune` | Manages the [images](lifecycle.md#fast-vms-images) new VMs are copied from |
| `limabean autostop on\|off\|status\|run` | Stops idle VMs and deletes expired ones every 10 minutes in the background, through a launchd agent ([details](lifecycle.md#idle-vms-stop-by-themselves)) |
| `limabean secrets ls\|set\|rm\|edit\|sync` | Manages the environment variables and secrets VMs get ([details](secrets.md)) |
| `limabean github login\|status\|sync\|logout` | Manages the GitHub token VMs use ([details](github.md)) |
| `limabean network ls\|allow\|rm\|open\|lock\|unlock` | Manages a VM's [network allowlist and lock](network.md) |
| `limabean network learn\|log` | Records what a VM looks up to [build its allowlist](network.md#learn-what-a-vm-needs), and shows what was blocked |
| `limabean claude sync` | Registers running VMs in Claude Desktop and removes stopped or deleted ones |
| `limabean mcp` | Runs an MCP server so Claude can manage VMs ([details](mcp.md)) |
| `limabean completion <shell>` | Prints a tab-completion script for bash, zsh, fish or PowerShell |
| `limabean version` | Prints the version |

Leave out a VM name and you pick one from a list.

## Interactive and plain mode

In a terminal, limabean shows its interface:
- **Spinners** for slow steps. Lima's output is shown only if a step fails.
- **Pickers and forms** for anything you didn't pass on the command line.
- **Confirmations** before `rm`, `rebuild` and `snapshot restore`; `rm` and
  `rebuild` also [check for unsaved work](lifecycle.md#unsaved-work-is-protected).
- **Tables** for `ls` and `secrets ls`.

When output isn't a terminal (pipes, scripts, CI), or with `--plain` or
`LIMABEAN_PLAIN=1`, it prints plain text and never prompts: missing names
are errors, and questions take their defaults.

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
PACKS="go,python,node,kubernetes,infra,containers,db"   # packs new VMs get; flag: new --with/--without
IMAGES="yes"                     # copy new VMs from a provisioned image; no: provision each one
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
