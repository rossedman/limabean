# limabean

Disposable Linux dev VMs on macOS, built from layered [Lima](https://lima-vm.io)
templates. Each VM comes with a full toolchain (Go, Python, containers, kubectl,
Terraform, …), uses your host git identity, and pushes with your host SSH keys
without copying them into the VM. You reach a VM with `ssh lima-<name>`, which
also makes it a target for remote tools such as Claude Code's SSH connections.

See [SPEC.md](SPEC.md) for the design.

## Requirements

`limabean doctor` checks all of these and prints the fix for anything missing.
`limabean init` can make the SSH config change for you.

### 1. Lima 2.0 or newer

```bash
brew install lima
```

### 2. SSH config: include Lima's host entries

Lima writes an SSH config for each VM to `~/.lima/<name>/ssh.config` and
rewrites it on every start, because the forwarded port changes. Include those
files from your own `~/.ssh/config` so every VM is reachable as `lima-<name>`:

```
Include ~/.lima/*/ssh.config
```

Put this line **at the top** of `~/.ssh/config`, before any `Host` or `Match`
block. An `Include` placed inside a block only applies to that block.

Check it with `ssh -G lima-scratch | grep -i hostname` once a VM exists. It
should print `hostname 127.0.0.1`.

### 3. SSH agent with your key loaded

VMs never receive a copy of your private keys. Lima forwards your Mac's
ssh-agent instead (`ssh.forwardAgent: true`), so git inside the VM can
authenticate as you only while the key is loaded in that agent.

Load your key once, storing the passphrase in the macOS keychain:

```bash
ssh-add --apple-use-keychain ~/.ssh/id_ed25519
```

Then, so it is loaded again after a reboot, add this to `~/.ssh/config`
(below the `Include` line):

```
Host *
  AddKeysToAgent yes
  UseKeychain yes
```

Check it with `ssh-add -l`. It should list your key, not "The agent has no
identities."

If you use another agent, such as 1Password or Secretive, make sure
`SSH_AUTH_SOCK` points at it in the shell where you run `limabean new`.
Forwarding uses whichever agent that variable names.

### 4. Your git config in a directory

Lima mounts directories, not single files, so each VM mounts one host
directory read-only at `/mnt/host/git` and its `~/.gitconfig` includes the
config found there. By default that directory is `~/.config/git`, which is
git's own XDG location. Git on the Mac reads it too.

| Your setup | What to do |
|---|---|
| Config already at `~/.config/git/config` | Nothing. |
| Config at `~/.gitconfig` | `mkdir -p ~/.config/git && mv ~/.gitconfig ~/.config/git/config` (git still finds it) |
| Config in a dotfiles repo | Nothing: `limabean init` follows the `~/.gitconfig` symlink and suggests that directory. Or set `GIT_CONFIG_DIR` in `~/.config/limabean/config` |

Files the VM looks for in that directory, first match wins:

- config: `config`, `.gitconfig`
- global ignore: `ignore`, `.gitignore`
- commit template: `.gitcommit.txt`, `commit-template.txt`

Edits on the Mac show up in every VM straight away. Settings that only make
sense on the host are overridden inside the VM (`core.editor` becomes `vim`).

## Quick start

```bash
git clone https://github.com/rossedman/limabean ~/code/github.com/rossedman/limabean
ln -s ~/code/github.com/rossedman/limabean/bin/limabean ~/.local/bin/limabean   # any directory on your PATH

limabean init          # save your settings, fix ~/.ssh/config, run doctor
limabean new dev       # create a VM called "dev" (takes a minute or two the first time)
limabean ssh dev       # or: ssh lima-dev
```

`limabean init` asks two things, the host directory with your git config and
where repositories go inside VMs, then runs `limabean doctor`, which checks
every requirement above and prints the exact fix for anything missing.

Check that git push will work from the VM:

```bash
limabean ssh dev ssh -T git@github.com
```

It should greet you by your GitHub username.

## The `limabean` command

| Command | What it does |
|---|---|
| `limabean init` | Writes `~/.config/limabean/config`, offers to add the `Include` to `~/.ssh/config` (with a backup), then runs `doctor` |
| `limabean doctor` | Checks Lima, the SSH `Include` and where it sits, agent keys and the git config directory |
| `limabean new <name> [env]` | Creates a VM from `templates/<env>.yaml` (default `scratch`) using your config |
| `limabean ls` | Lists limabean VMs with status, environment, size and SSH host |
| `limabean ssh <name> [cmd…]` | Opens a shell or runs a command, starting the VM first if it's stopped |
| `limabean get <name> [-p] <repo>` | `ghq get` inside the VM, e.g. `limabean get dev rossedman/limabean` |
| `limabean stop <name>` | Stops a VM |
| `limabean rebuild <name>` | Deletes and recreates the VM from the current templates, with the same environment |
| `limabean rm <name>` | Deletes a VM |
| `limabean claude sync` | Adds every VM to Claude Desktop as an SSH connection and removes entries for deleted VMs |

**Your work is protected.** `rm` and `rebuild` first look at every git
repository under `~/code` in the VM. They refuse to run if any repository has
uncommitted changes, stashes or commits not pushed to a remote, and list what
they found. `--force` skips the check.

Settings live in `~/.config/limabean/config`, a small shell file:

```bash
GIT_CONFIG_DIR="~/.dotfiles/git"   # host directory mounted read-only into VMs
CODE_DIR="~/code"                  # where ghq clones inside VMs
CPUS=""                            # empty: template default (4)
MEMORY=""                          # GiB; empty: template default (8)
CLAUDE_DESKTOP="yes"               # add VMs to Claude Desktop as SSH connections
```

Changes apply to VMs created afterwards, or to existing ones after
`limabean rebuild`. Everything `limabean` does is plain `limactl`, so you can
always use `limactl` directly.

## Connecting Claude

`limabean new` adds every VM to the Claude desktop app as an SSH connection,
and `limabean rm` removes it. In the prompt box's environment dropdown, pick
**SSH → `<name> (limabean)`**, then open a directory under `~/code` in the VM.
Commands Claude runs there use the VM's tools and can't touch files on your
Mac.

How it works: Claude Desktop reads connections from the `sshConfigs` list in
`~/.claude/settings.json` ([docs](https://code.claude.com/docs/en/desktop)).
limabean adds an entry per VM whose `sshHost` is the `lima-<name>` alias from
your SSH config, and only ever touches entries with ids starting `limabean-`.
Your other settings are left alone, and the file isn't changed if it isn't
valid JSON. Editing it needs `jq` (`brew install jq`).

- A VM deleted with `limactl delete` instead of `limabean rm` loses its entry
  the next time you run `limabean ls` or `limabean new`. `limabean rm <name>`
  also cleans it up, even though the VM is already gone.
- VMs created before this feature, or out of sync: `limabean claude sync`
  adds every VM and removes entries for deleted ones.
- Restart Claude Desktop if a new VM doesn't show up in the dropdown.
- Claude Desktop installs its own copy of Claude Code in the VM on first
  connect, under `~/.claude/remote/` (about 1 GB). Nothing needs to be
  provisioned for it.
- To turn this off, set `CLAUDE_DESKTOP="no"` in `~/.config/limabean/config`.

## What's in a VM

Ubuntu 24.04, with:

- **zsh** as the login shell, with autosuggestions, syntax highlighting, fzf
  keybindings and a single-line prompt (see [Shell](#shell)).
- **apt:** git, git-extras, git-quick-stats, curl, wget, vim, tmux, stow,
  httpie, jq, tree, htop, watch, speedtest-cli, build-essential, python3, pip,
  venv, skopeo
- **Docker Engine** from Docker's apt repository, with the buildx and compose
  plugins. BuildKit is the default builder, so custom Dockerfile syntaxes
  (`# syntax=docker/dockerfile:1`) and other BuildKit features work. Your user
  owns the socket, so `docker` needs no sudo, and kind, testcontainers and
  other tools that expect a Docker daemon work as they do on a Mac.
- **mise** ([templates/mise/base.toml](templates/mise/base.toml), all
  versions pinned): go, goreleaser, golangci-lint, uv, kubectl, helm, kind,
  k9s, terraform, aws-cli, cosign, syft, grype, ripgrep, fd, fzf, bat, gh, ghq,
  yq, direnv (hooked into bash), glow, gum, shellcheck, svu
- **`~/.editorconfig`** from
  [templates/files/editorconfig](templates/files/editorconfig). A project's
  own `.editorconfig` takes precedence.

Use mise for multiple versions: `mise use python@3.12` (instead of pyenv),
`mise use terraform@1.9` (instead of tfswitch).

The VM has no access to your Mac's files apart from the read-only git config
mount. Keep code inside the VM (`~/code`) and push it to a remote.

## Shell

zsh is the login shell. The prompt is built from zsh's own `vcs_info`, with no
prompt framework:

```
scratch:~/c/g/r/limabean (main*) ❯
```

It shows the VM name, the path with parent directories shortened to one letter,
and the git branch with `*` for unstaged, `+` for staged and `?` for untracked
changes. `❯` turns red after a failed command.

Also included:
- history shared across sessions
- autosuggestions as you type
- syntax highlighting
- fzf on `ctrl-r` (history), `ctrl-t` (files) and `alt-c` (directories)
- `repo [query]` to jump to any ghq repository with fzf
- `cat` is bat without the file header and without a pager. Piped output is
  plain, and the real cat is `command cat`. bat uses the `ansi` theme, so its
  colors follow your terminal's palette.
- tab completion for almost every installed tool. Most is generated at boot
  into `~/.local/share/zsh/completions` by
  [completions.sh](templates/features/completions.sh): gh, kubectl, helm, kind,
  k9s, docker, mise, go, goreleaser, golangci-lint, uv, pip, cosign,
  syft, grype, glow, gum, svu, rg, fd, bat, yq and ghq. aws and terraform use
  their own completers. git, ssh, make, tmux and others come with zsh.

### Paths

[env.sh](templates/files/env.sh) is loaded by every shell, interactive or not.
It sets:

| Variable | Value | Used by |
|---|---|---|
| `GOPATH` | `~/go` | Go module cache and workspace |
| `GOBIN` | `~/go/bin` | `go install` |
| `PYTHONUSERBASE` | `~/.local` | `pip install --user`, `uv tool install` and pipx, which put scripts in `~/.local/bin` |
| `PATH` | `$GOBIN`, `~/.local/bin`, mise shims, then the system | Each directory is added once, so nested shells don't grow it |

| File in the VM | Source | Notes |
|---|---|---|
| `~/.zshrc` | [templates/files/zshrc](templates/files/zshrc) | Rewritten on every boot. Put personal additions in `~/.zshrc.local`, which persists. |
| `~/.config/limabean/env.sh` | [templates/files/env.sh](templates/files/env.sh) | PATH and env vars for every shell, interactive or not. Sourced by `~/.zshenv` and `~/.bashrc`. |

## Repositories with ghq

[ghq](https://github.com/x-motemen/ghq) is configured to clone into `~/code`,
so repositories land at `~/code/<host>/<owner>/<repo>`:

```bash
ghq get rossedman/limabean        # → ~/code/github.com/rossedman/limabean
cd "$(ghq list -p | fzf)"
```

GitHub repositories are fetched over https and pushed over SSH, through
`pushInsteadOf` and your forwarded agent. Public repositories clone without a
key. For private ones use `ghq get -p` (SSH) so the clone itself authenticates.

To use another directory, set `CODE_DIR` in `~/.config/limabean/config`. A
`[ghq "<url>"] root = …` in your own git config still applies to that URL.

## Changing things

| To… | Edit |
|---|---|
| Add or upgrade a CLI tool | [templates/mise/base.toml](templates/mise/base.toml). Pin a version; `mise latest <tool>` in a VM shows the newest. |
| Change where ghq clones | `CODE_DIR` in `~/.config/limabean/config` |
| Change the prompt, aliases or shell options | [templates/files/zshrc](templates/files/zshrc) |
| Add a tool's zsh completion | a `gen` line in [templates/features/completions.sh](templates/features/completions.sh) |
| Add an environment variable for every shell | [templates/files/env.sh](templates/files/env.sh) |
| Change the global editorconfig | [templates/files/editorconfig](templates/files/editorconfig) |
| Add a system package | `packages=(…)` in [templates/features/apt-common.sh](templates/features/apt-common.sh) |
| Change CPU or memory | `CPUS`/`MEMORY` in `~/.config/limabean/config`, or the defaults in [templates/layer-os.yaml](templates/layer-os.yaml) |
| Change disk size | [templates/layer-os.yaml](templates/layer-os.yaml) |
| Add a new environment | a new `templates/<env>.yaml` with `base: [./layer-base.yaml]` |

Then rebuild. VMs are disposable, and rebuilding is how changes get applied:

```bash
limabean rebuild dev
```

Lima doesn't allow `..` in relative paths or base template file names starting
with `_`. That's why scripts and TOML files live under `templates/` and the
layers are named `layer-*.yaml`.

CI ([.github/workflows/lint.yml](.github/workflows/lint.yml)) runs on every push
and pull request: `shellcheck` over `bin/limabean` and `templates/features/*.sh`,
and `limactl tmpl validate` on every `templates/*.yaml` except the `layer-*`
bases. It doesn't boot a VM.

## Troubleshooting

Start with `limabean doctor`.


| Symptom | Cause |
|---|---|
| `ssh: Could not resolve hostname lima-<name>` | The `Include` line is missing, or it sits inside a `Host` block (requirement 2). |
| `git@github.com: Permission denied (publickey)` in the VM | No key in your Mac's agent: `ssh-add -l` (requirement 3). |
| `limabean: no git config found` in the boot log | `gitConfigDir` points at a directory without a config file (requirement 4). |
| Start hangs on "mise tools are installed" | Check the provisioning log: `limactl shell <name> sudo tail -50 /var/log/cloud-init-output.log` |
