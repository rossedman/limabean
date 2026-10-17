# The `limabean` command

`limabean` is a thin wrapper around `limactl`. Everything it does is plain
Lima, so `limactl` always works alongside it.

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
| `limabean github login\|status\|sync\|logout` | Manages the GitHub token that VMs use for `gh` and the API ([details](github.md)) |
| `limabean claude sync` | Registers every VM in Claude Desktop and removes entries for deleted VMs |

## Unsaved work is protected

`rm` and `rebuild` first check every git repository under `~/code` in the VM.
If any has uncommitted changes, stashes or commits not pushed to a remote, they
stop and list what they found. `--force` skips the check.

## Settings

`~/.config/limabean/config` is a small shell file written by `init`:

```bash
GIT_CONFIG_DIR="~/.config/git"   # host directory mounted read-only into VMs
CODE_DIR="~/code"                # where ghq clones inside VMs
CPUS=""                          # empty: template default (4)
MEMORY=""                        # GiB; empty: template default (8)
CLAUDE_DESKTOP="yes"             # register VMs as SSH connections in Claude Desktop
```

Changes apply to new VMs, and to existing ones after `limabean rebuild`.
