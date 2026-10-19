# The `limabean` command

`limabean` is a single Go binary that drives `limactl`. Everything it does is
plain Lima, so `limactl` keeps working alongside it. Every command has full
help: `limabean <command> --help` explains it and shows examples.

| Command | What it does |
|---|---|
| `limabean init` | Walks through first-time setup (git config directory, code directory, the SSH `Include`, an optional GitHub token), then runs `doctor`. `--project` writes a [`.limabean.yaml`](projects.md#your-first-project-file) for the repository you're in instead, from what's in it (`--print` to show it, `--overwrite` to replace one, `--dir` for another checkout) |
| `limabean config` | Shows every setting, its value, and whether it came from a flag, a `LIMABEAN_*` variable, the config file or the default (`--json` for scripts) |
| `limabean doctor` | Checks Lima, the SSH `Include` and its position, agent keys, git config, Claude Desktop, the GitHub token and every secret reference; in a project, that its file's packs, network presets and secrets exist on this Mac, and whether its VM matches it |
| `limabean bump [file…]` | Updates the pinned versions in mise TOML files (default `templates/packs/*.toml`) within each major, with `mise latest` on the Mac or `--vm` |
| `limabean recipes [name]` | Lists the starter [recipes](recipes.md), or writes one to `.limabean.yaml` here (`--print` to show it, `--overwrite` to replace a project file) |
| `limabean up [name] [env]` | Brings a VM up. In a project, without a name (or with the project's): creates the VM its [`.limabean.yaml`](projects.md) describes (clones the repository, turns on its Claude plugins, runs its setup), or starts it and [applies the file's changes](projects.md#changing-the-file), then runs its [`start`](projects.md#start) in the background; `--setup` runs the setup again, and `--preview` shows what `up` would create or change without changing anything. With another name: starts that VM, adding it back to Claude Desktop, or creates it; in a terminal a form asks which [packs](packs.md) and secrets it gets, `--with`/`--without` choose packs instead, `--allow`, `--locked` and `--learn` set its [network policy](network.md), and `--ttl` [deletes it later](lifecycle.md#throwaway-vms) |
| `limabean ps` | Shows the project's VM: running or not, whether its `start` runs, the ports in it and whether each reaches your Mac, and what `up` would change; `--json` for scripts |
| `limabean info [name]` | Shows what [`limabean-info`](inside-a-vm.md#what-this-vm-is) prints in a running VM: packs, tools, network, secrets' names, and the project's clone, ports and logs; `--json` for scripts |
| `limabean logs [name]` | Shows the end of the VM's provisioning log, or with `--setup` / `--start` its project's setup or start output, or with `--lima` Lima's own log on the Mac; `-n` lines, `-f` to follow |
| `limabean down [name]` | Stops the project's VM, or the one named, and removes it from Claude Desktop until it starts again; `--idle 2h` stops only VMs [nobody is using](lifecycle.md#idle-vms-stop-by-themselves) |
| `limabean run [flags] -- <cmd>` | Creates a VM, runs one command in it (in a fresh clone with `--repo`), and with `--rm` deletes it after; exits with the command's status ([details](lifecycle.md#throwaway-vms)) |
| `limabean packs` | Lists the [packs](packs.md) of tools a VM can get; `--json` for scripts |
| `limabean ls` | Lists VMs with status, environment, size, number of secrets, network and SSH host, and when throwaway ones expire; `--json` prints them for scripts |
| `limabean ssh [name] [cmd…]` | Opens a shell or runs a command, starting a stopped VM first; in a project's checkout, in the VM's clone of the repository (`limabean ssh -- make test`) |
| `limabean get [name] <repo>` | Clones a repository into `~/code` with ghq; flags after the VM name (like `-p`) go to ghq |
| `limabean cp <src>… <dst>` | Copies files between the Mac and VMs; VM paths are `<vm>:<path>`, `-r` for directories |
| `limabean code [name] [repo]` | Opens the VM's `~/code`, or one clone in it, in VS Code (or Insiders), Cursor, Windsurf, VSCodium or another editor with VS Code's `--remote` flag, over SSH; `--ide` picks the editor, and a missing Remote - SSH extension is offered for install |
| `limabean remote [name]` | Adds the VM's clone of the repository you're in as a git remote of your Mac checkout (`limabean` by default, `--name` for another), so `git fetch limabean` brings its branches to the Mac without pushing them to GitHub; branches only, no tags; `--rm` removes it ([details](projects.md#review-on-your-mac)) |
| `limabean ports [name]` | Lists the TCP ports programs in the VM listen on and whether each reaches your Mac's localhost; `--json` for scripts |
| `limabean open [name] [port]` | Opens `http://localhost:<port>` in your browser; without a port, the VM's only forwarded one, or a picker |
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
| `limabean version` | Prints the version (`--json` for scripts) |
| `limabean help [command]` | Shows help; `--json` prints the command tree, with every command's usage and flags, for scripts and agents |

Leave out a VM name and you pick one from a list. Inside a project, `ssh`,
`code`, `get`, `info`, `remote`, `open`, `ports`, `network ls` and `snapshot save`/`ls` use
[the project's VM](projects.md#every-day) instead; `rm`, `rebuild` and
`snapshot restore` always need a name.

## Interactive and plain mode

In a terminal, limabean shows its interface:
- **Spinners** for slow steps. Lima's output is shown only if a step fails.
- **Pickers and forms** for anything you didn't pass on the command line.
- **Confirmations** before `rm`, `rebuild` and `snapshot restore`; `rm` and
  `rebuild` also [check for unsaved work](lifecycle.md#unsaved-work-is-protected).
  `-y`/`--yes` answers the question; on `rm` and `rebuild`, only `--force`
  skips the unsaved-work check as well.
- **Tables** for `ls` and `secrets ls`.

When output isn't a terminal (pipes, scripts, CI), or with `--plain`,
`LIMABEAN_PLAIN=1` or `TERM=dumb`, it prints plain text and never prompts:
missing names are errors, and questions take their defaults.

## Output

- **stdout carries only data:** tables, `--json`, the reports of `ps`,
  `doctor` and the `status` commands, `recipes --print`, `logs`. Status
  lines, progress, hints and prompts go to stderr, so
  `limabean ls --json | jq` and `limabean ps > state.txt` get only the
  result.
- **An empty result** prints nothing on stdout (`[]` with `--json`), and a
  note on stderr in a terminal.
- **Color only adds**: every colored mark also has a word or symbol. It's
  off with `NO_COLOR=1`, `--no-color`, `TERM=dumb`, or when output isn't a
  terminal.
- `-q`/`--quiet` prints only errors, and the data a command returns.
  `-v`/`--verbose` also prints each `limactl` and ssh command limabean runs;
  with both, `--verbose` wins.
- **New releases**: once a day, in a terminal, limabean checks GitHub for a
  newer release in the background and says so after a command:
  `limabean 0.4.0 is out: brew upgrade limabean`. Never in scripts,
  `--json`, `-q` or MCP; `UPDATE_CHECK=no` turns it off.
- `limabean help --json` describes every command and flag, and
  `limabean version --json` the version, Go version and platform.

## Flag names

Flags are named for what happens:
- `--preview` (on `up`, `down --idle` and `bump`) shows what would change
  and changes nothing.
- `-y`/`--yes` skips a confirmation, `--force` a safety check, and
  `--overwrite` lets `recipes` replace a file.
- `ls` commands also answer to `list`, and `rm` commands to `delete` (or
  `remove`).

The earlier names `--dry-run`, `recipes --force` and
`snapshot restore --force` still work, but aren't shown in help. So do the
commands `new`, `create`, `start` and `stop`, which print a note naming the
`up` or `down` command that replaced them.

After the form `up` shows for a new VM, it prints the same command with
flags, to use in scripts. limabean writes its own files (settings, secret
references, state) atomically, so an interrupted write never leaves half a
file.

## Exit codes

Scripts can rely on these; they won't be renumbered.

| Code | Means |
|---|---|
| 0 | It worked, or there was nothing to do |
| 1 | Something else went wrong; the message says what |
| 2 | A bad flag or argument, or a value missing without a terminal to ask (`--yes`, a VM name) |
| 3 | Reserved for authentication and permission failures |
| 4 | The VM, project file, recipe or other target doesn't exist |
| 5 | It's in the way: a VM of that name exists, a file would be replaced, or [unsaved work](lifecycle.md#unsaved-work-is-protected) stopped `rm` or `rebuild` |
| 130 | You cancelled with Ctrl-C |

`limabean run` exits with its command's own status instead.

With `--json`, a failure prints a JSON object on stderr rather than a line
of text, so a script parsing the output gets something it can read:

```json
{"error": {"code": "not_found", "exit": 4, "message": "no VM named \"api\" (see: limabean ls)"}}
```

If limabean itself crashes, it says so and prints what to include in a
[bug report](https://github.com/rossedman/limabean/issues/new).

## Settings

`~/.config/limabean/config` is a small `KEY="value"` file written by `init`.
Each setting can also come from a `LIMABEAN_*` environment variable, or a
flag where one exists, which take priority. `limabean config` shows each
value and where it came from:

```bash
GIT_CONFIG_DIR="~/.config/git"   # host directory mounted read-only into VMs
CODE_DIR="~/code"                # where ghq clones inside VMs
CODE_DISK="50"                   # GiB of the disk CODE_DIR lives on, kept by rebuild; empty: none
CPUS=""                          # empty: template default (2); flag: up --cpus
MEMORY=""                        # GiB; empty: template default (4); flag: up --memory
DISK=""                          # GiB of system disk; empty: template default (10); flag: up --disk
CLAUDE_DESKTOP="yes"             # register VMs as SSH connections in Claude Desktop
AWS_DIR="~/.aws"                 # mounted read-only at ~/.aws in VMs if it exists; empty: never
PACKS=""                         # packs every new VM gets besides core; empty: none (new asks);
                                 # flag: up --with/--without; project files ignore it
IMAGES="yes"                     # copy new VMs from a provisioned image; no: provision each one
CA_CERTS="yes"                   # VMs trust root CAs an admin added to the Mac (a company proxy's);
                                 # no: only Ubuntu's own list
UPDATE_CHECK="yes"               # say in a terminal when a newer limabean is out; no: never check
IDE=""                           # editor limabean code opens: code, code-insiders, cursor, windsurf, codium, …;
                                 # empty: the first installed; flag: code --ide
```

Changes apply to new VMs, and to existing ones after `limabean rebuild`.
IDE is the exception: it only affects `limabean code`, so it applies at once.
`limabean code` finds an editor on your PATH or inside its app in
`/Applications`, so VS Code's "Install 'code' command in PATH" isn't needed.
If the IDE you set isn't installed, it opens the first editor that is and
says how to make that the default.

The older `new` (or `create`), `start` and `stop` still work, for scripts
and habit: `new` and `start` are `up <name>`, `stop` is `down <name>`.

Global flags:

| Flag | |
|---|---|
| `--config <file>` | Use another config file |
| `--plain` | Plain text, no prompts (`LIMABEAN_PLAIN=1`) |
| `--no-color` | No color (`NO_COLOR=1`) |
| `-q`, `--quiet` | Only errors, and the data a command returns |
| `-v`, `--verbose` | Also the `limactl` and ssh commands limabean runs |
| `--templates <dir>` | Read templates from a directory instead of the built-in copy (`LIMABEAN_TEMPLATES`); see [Customizing](customizing.md) |
