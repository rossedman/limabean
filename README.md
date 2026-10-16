# limabean

Disposable Linux dev VMs on macOS, built from layered [Lima](https://lima-vm.io)
templates. Each VM comes with a full toolchain (Go, Python, containers, kubectl,
Terraform, …), uses your host git identity, and pushes with your host SSH keys
without copying them into the VM. You reach a VM with `ssh lima-<name>`, which
also makes it a target for remote tools such as Claude Code's SSH connections.

See [SPEC.md](SPEC.md) for the design.

## Requirements

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
`SSH_AUTH_SOCK` points at it in the shell where you run `limactl start`.
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
| Config in a dotfiles repo | Pass the directory when creating a VM: `--param gitConfigDir=~/dotfiles/git` |

Files the VM looks for in that directory, first match wins:

- config: `config`, `.gitconfig`
- global ignore: `ignore`, `.gitignore`
- commit template: `.gitcommit.txt`, `commit-template.txt`

Edits on the Mac show up in every VM straight away. Settings that only make
sense on the host are overridden inside the VM (`core.editor` becomes `vim`).

## Quick start

```bash
limactl start --name=scratch templates/scratch.yaml
ssh lima-scratch
```

With a custom git config directory:

```bash
limactl start --name=scratch --param gitConfigDir=~/dotfiles/git templates/scratch.yaml
```

The first start takes about a minute: it downloads the Ubuntu image, then
installs apt packages, containerd and the mise tools. Later starts take seconds.

Check that git push will work from the VM:

```bash
ssh lima-scratch ssh -T git@github.com
```

It should greet you by your GitHub username.

## Connecting Claude

Nothing Claude-specific runs inside the VM. In the Claude desktop app, add an
SSH connection to host `lima-<name>` (for example `lima-scratch`) and open a
directory under `~/code` in the VM. Commands Claude runs there use the VM's
tools and can't touch files on your Mac.

## What's in a VM

Ubuntu 24.04, with:

- **zsh** as the login shell, with autosuggestions, syntax highlighting, fzf
  keybindings and a single-line prompt (see [Shell](#shell)).
- **apt:** git, git-extras, git-quick-stats, curl, wget, vim, tmux, stow,
  httpie, jq, tree, htop, watch, speedtest-cli, build-essential, python3, pip,
  venv, skopeo
- **Containers:** rootless containerd, nerdctl and BuildKit, installed and
  managed by Lima. `docker` is a symlink to `nerdctl`, so `docker run`,
  `docker build` and `docker compose` work. Tools that need a real Docker
  daemon socket won't. kind runs with `KIND_EXPERIMENTAL_PROVIDER=nerdctl`.
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

To use another directory, create the VM with `--param codeDir=~/src`. A
`[ghq "<url>"] root = …` in your own git config still applies to that URL.

## Changing things

| To… | Edit |
|---|---|
| Add or upgrade a CLI tool | [templates/mise/base.toml](templates/mise/base.toml). Pin a version; `mise latest <tool>` in a VM shows the newest. |
| Change where ghq clones | `codeDir` param in [templates/layer-base.yaml](templates/layer-base.yaml), or per VM: `--param codeDir=…` |
| Change the prompt, aliases or shell options | [templates/files/zshrc](templates/files/zshrc) |
| Add an environment variable for every shell | [templates/files/env.sh](templates/files/env.sh) |
| Change the global editorconfig | [templates/files/editorconfig](templates/files/editorconfig) |
| Add a system package | `packages=(…)` in [templates/features/apt-common.sh](templates/features/apt-common.sh) |
| Change CPU, memory or disk | [templates/layer-os.yaml](templates/layer-os.yaml), or per VM: `limactl start --cpus 8 --memory 16 …` |
| Add a new environment | a new `templates/<env>.yaml` with `base: [./layer-base.yaml]` |

Then rebuild. VMs are disposable, and rebuilding is how changes get applied:

```bash
limactl delete -f scratch
limactl start --name=scratch templates/scratch.yaml
```

Lima doesn't allow `..` in relative paths or base template file names starting
with `_`. That's why scripts and TOML files live under `templates/` and the
layers are named `layer-*.yaml`.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `ssh: Could not resolve hostname lima-<name>` | The `Include` line is missing, or it sits inside a `Host` block (requirement 2). |
| `git@github.com: Permission denied (publickey)` in the VM | No key in your Mac's agent: `ssh-add -l` (requirement 3). |
| `limabean: no git config found` in the boot log | `gitConfigDir` points at a directory without a config file (requirement 4). |
| Start hangs on "mise tools are installed" | Check the provisioning log: `limactl shell <name> sudo tail -50 /var/log/cloud-init-output.log` |
