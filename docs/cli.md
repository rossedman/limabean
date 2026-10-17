# The `limabean` command

`limabean` is a single Go binary that drives `limactl`. Everything it does is
plain Lima, so `limactl` keeps working alongside it. Every command has full
help: `limabean <command> --help` explains it and shows examples.

| Command | What it does |
|---|---|
| `limabean init` | Walks through first-time setup (git config directory, code directory, the SSH `Include`, an optional GitHub token), then runs `doctor` |
| `limabean doctor` | Checks Lima, the SSH `Include` and its position, agent keys, git config, Claude Desktop, the GitHub token and every secret reference |
| `limabean new [name] [env]` | Creates a VM. In a terminal, a form asks for missing values and which secrets the VM gets |
| `limabean ls` | Lists VMs with status, environment, size, number of secrets and SSH host; `--json` prints them for scripts |
| `limabean ssh [name] [cmd…]` | Opens a shell or runs a command, starting a stopped VM first |
| `limabean get [name] <repo>` | Clones a repository into `~/code` with ghq; flags after the VM name (like `-p`) go to ghq |
| `limabean start [name]` | Starts a stopped VM and adds it back to Claude Desktop |
| `limabean stop [name]` | Stops a VM and removes it from Claude Desktop until it starts again |
| `limabean rebuild [name]` | Recreates the VM from the current templates, keeping its environment and secrets |
| `limabean rm [name]` | Deletes a VM, its Claude Desktop entry and its own secrets |
| `limabean secrets ls\|set\|rm\|edit\|sync` | Manages the environment variables and secrets VMs get ([details](secrets.md)) |
| `limabean github login\|status\|sync\|logout` | Manages the GitHub token VMs use ([details](github.md)) |
| `limabean claude sync` | Registers running VMs in Claude Desktop and removes stopped or deleted ones |
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

`rm` and `rebuild` first check every git repository under `~/code` in the
VM. If any has uncommitted changes, stashes or commits not pushed to a
remote, they stop and list what they found. Then, in a terminal, they ask
before deleting. `--force` skips both.

## Settings

`~/.config/limabean/config` is a small `KEY="value"` file written by `init`.
Each setting can also come from a `LIMABEAN_*` environment variable, or a
flag where one exists, which take priority:

```bash
GIT_CONFIG_DIR="~/.config/git"   # host directory mounted read-only into VMs
CODE_DIR="~/code"                # where ghq clones inside VMs
CPUS=""                          # empty: template default (4); flag: new --cpus
MEMORY=""                        # GiB; empty: template default (8); flag: new --memory
CLAUDE_DESKTOP="yes"             # register VMs as SSH connections in Claude Desktop
AWS_DIR="~/.aws"                 # mounted read-only at ~/.aws in VMs if it exists; empty: never
```

Changes apply to new VMs, and to existing ones after `limabean rebuild`.

Global flags:

| Flag | |
|---|---|
| `--config <file>` | Use another config file |
| `--plain` | Plain text, no prompts (`LIMABEAN_PLAIN=1`) |
| `--templates <dir>` | Read templates from a directory instead of the built-in copy (`LIMABEAN_TEMPLATES`); see [Customizing](customizing.md) |
