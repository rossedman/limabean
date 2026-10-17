# The `limabean` command

`limabean` is a thin wrapper around `limactl`. Everything it does is plain
Lima, so `limactl` always works alongside it.

In a terminal it uses [gum](https://github.com/charmbracelet/gum) for its
interface:
- slow steps (creating, starting, stopping and deleting VMs) run behind a
  spinner, and Lima's output is shown only if a step fails
- leave out a VM name and you pick from a list: `limabean ssh`, `stop`, `rm`,
  `rebuild` and `get <repo>`. `limabean new` asks for a name.
- `init` and `github login` prompt for their values
- `rm` and `rebuild` ask before deleting

When output isn't a terminal (pipes, scripts, CI), or with `LIMABEAN_PLAIN=1`,
it prints plain text and never prompts: missing names are errors, and
questions take their defaults.

| Command | What it does |
|---|---|
| `limabean init` | Saves your settings to `~/.config/limabean/config`, offers to add the SSH `Include` to `~/.ssh/config` (with a backup), then runs `doctor` |
| `limabean doctor` | Checks Lima, the SSH `Include` and its position, agent keys, the git config directory and `jq` |
| `limabean new <name> [env]` | Creates a VM from `templates/<env>.yaml` (default `scratch`) and registers it in Claude Desktop |
| `limabean ls` | Lists limabean VMs with status, environment, size and SSH host |
| `limabean ssh <name> [cmd…]` | Opens a shell or runs a command, starting the VM first if it's stopped |
| `limabean get <name> [-p] <repo>` | Runs `ghq get` in the VM, e.g. `limabean get dev rossedman/limabean`. `-p` clones over SSH |
| `limabean stop <name>` | Stops a VM |
| `limabean rebuild <name> [--force]` | Deletes and recreates the VM from the current templates, keeping its environment |
| `limabean rm <name> [--force]` | Deletes a VM and its Claude Desktop entry |
| `limabean github login [--with-token]` | Saves a GitHub token for VMs to the Keychain. `--with-token` reads it from stdin, e.g. `gh auth token \| limabean github login --with-token` ([details](github.md)) |
| `limabean github status\|sync\|logout` | Shows, reinstalls or removes the GitHub token ([details](github.md)) |
| `limabean claude sync` | Registers every VM in Claude Desktop and removes entries for deleted VMs |

## Unsaved work is protected

`rm` and `rebuild` first check every git repository under `~/code` in the VM.
If any has uncommitted changes, stashes or commits not pushed to a remote, they
stop and list what they found. Then, in a terminal, they ask before deleting.
`--force` skips both.

## Settings

`~/.config/limabean/config` is a small shell file written by `init`:

```bash
GIT_CONFIG_DIR="~/.config/git"   # host directory mounted read-only into VMs
CODE_DIR="~/code"                # where ghq clones inside VMs
CPUS=""                          # empty: template default (4)
MEMORY=""                        # GiB; empty: template default (8)
CLAUDE_DESKTOP="yes"             # register VMs as SSH connections in Claude Desktop
AWS_DIR="~/.aws"                 # mounted read-only at ~/.aws in VMs if it exists; empty: never
```

Changes apply to new VMs, and to existing ones after `limabean rebuild`.
