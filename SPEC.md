# limabean — spec (draft)

Reproducible Lima VMs that act as personal dev environments, one per purpose,
usable by anyone on macOS. Claude, or any SSH client, connects to them over SSH
and works inside them.

**Main goal: easy to configure and maintain.** If a decision makes it harder to
add an environment or change a tool once for all of them, it's the wrong
decision.

## Goals

1. Defining a new environment means writing about 10 lines of YAML.
2. Each tool, dotfile or setting is defined in exactly one place.
3. VMs can be thrown away. Rebuilding one is one command and nothing is lost.
4. Claude, running on the Mac, can reach any environment over SSH by a stable
   host name.
5. It works with Lima as it ships (2.0 or newer). No extra layers such as Nix
   or Ansible unless they clearly earn their place.
6. It isn't tied to one person's machine. Host-specific values are template
   params with sensible defaults, and host prerequisites are documented in
   README.md.

## Non-goals

- Supporting other operating systems or hypervisors (macOS host, Lima only).
- Managing the user's dotfiles. limabean reads host config read-only and
  never owns it.
- Production or CI images.

## Host prerequisites

These are documented for users in README.md:

| Prerequisite | Why |
|---|---|
| Lima 2.0 or newer | `base:` template composition and `param:` |
| `Include ~/.lima/*/ssh.config` at the top of `~/.ssh/config` | every VM is reachable as `ssh lima-<name>`, which survives port changes |
| A key loaded in the host ssh-agent (`AddKeysToAgent`/`UseKeychain` to persist it) | agent forwarding is the only way the VM can authenticate as the user |
| Git config in a directory (`~/.config/git` by default, or the `gitConfigDir` param) | Lima mounts directories, not files |

## Architecture: layered templates

Lima templates can inherit from other templates through `base:`. Use that to
build three layers:

```
templates/
  layer-os.yaml       # layer 1: Ubuntu 24.04, CPU/mem/disk, ssh agent forwarding, containerd
  layer-base.yaml     # layer 2: base: layer-os + apt + mise tools + git + docker alias + zsh + editorconfig
  scratch.yaml        # layer 3: base: layer-base, nothing else (throwaway sandbox)
  go.yaml             #          base: layer-base + mise/go.toml
  python.yaml  k8s.yaml  ...
  features/           # idempotent shell scripts, one per capability
    apt-common.sh  mise.sh  git.sh  nerdctl.sh  shell.sh
  files/              # static files written into the VM (zshrc, env.sh, editorconfig)
  mise/               # tool versions; base.toml + one per environment
    base.toml  go.toml  python.toml
bin/limabean          # thin wrapper CLI (milestone 3)
```

Everything sits under `templates/` because Lima rejects relative paths that
contain `..` and base templates whose file names start with `_`.

An environment template stays tiny. It only ships its mise TOML:

```yaml
# templates/go.yaml
base:
- ./layer-base.yaml
cpus: 6
provision:
- mode: data
  path: /etc/limabean/mise/go.toml
  file: ./mise/go.toml
```

and the base layer carries everything that is shared:

```yaml
# templates/layer-base.yaml
base:
- ./layer-os.yaml
provision:
- mode: system
  file: ./features/apt-common.sh
- mode: data
  path: /etc/limabean/mise/00-base.toml
  file: ./mise/base.toml
- mode: user
  file: ./features/mise.sh          # installs mise, links /etc/limabean/mise/*.toml, mise install
```

### Why this layout

- **Feature scripts plus thin YAML.** Logic lives in plain shell scripts. The
  YAML only says which features an environment gets. Adding a tool to every
  environment means editing one file.
- **mise for toolchains (decided).** Each environment gets its own declarative
  `mise/<env>.toml` with pinned versions, so upgrading Go is a one-line diff.
  apt covers only system packages.
- **Nothing from the host is mounted.** `file:` references are embedded into
  the instance when it is created, and `mode: data` writes are guaranteed to
  happen before any provisioning script runs. That means provision order
  across layers doesn't matter: `mise.sh` installs every TOML that is present.
  Changing a template takes effect on `rebuild`.
- **Scripts run on every boot**, so each one is idempotent and exits quickly
  when there's nothing to do.
- **PATH for SSH commands.** `files/env.sh` (PATH with `~/.local/bin` and the
  mise shims, plus env vars) is sourced from `~/.zshenv`, which zsh reads even
  for non-interactive commands, and from the top of `~/.bashrc`. So commands run
  over SSH (`ssh lima-<name> go test ./...`) see the same tools as a login.
- **Shell.** zsh is the login shell (`features/shell.sh`). `files/zshrc` holds
  plain zsh config with a `vcs_info` prompt. There's deliberately no prompt
  framework or plugin manager: two apt plugins (autosuggestions, syntax
  highlighting) and fzf's built-in keybindings. Managed files are rewritten on
  every boot; personal additions go in `~/.zshrc.local`.

## Dotfiles: deferred

The first version covers provisioning only. When dotfiles come back, the plan is
to mount a dotfiles directory (a param, like `gitConfigDir`) read-only and
apply a Linux-safe subset in a
`features/dotfiles.sh` script. This needs D3 (shell config portability) to be
settled first.

## Wrapper CLI

Keep it small. A `justfile` or a ~100-line bash script:

```
limabean new go [--name go-api]   # limactl start --name=go-api templates/go.yaml
limabean ls                       # limactl list
limabean sh go-api                # limactl shell go-api (or ssh lima-go-api)
limabean rebuild go-api           # stop, delete, create from the current template
limabean update                   # re-run feature scripts in place (idempotent)
limabean lint                     # limactl validate + shellcheck features/*.sh
```

`rebuild` is the main maintenance path: change a template, rebuild, done. Avoid
patching long-lived VMs by hand.

## Claude over SSH

Claude is not provisioned into the VMs. Claude runs from the Mac and connects
over SSH, using the `lima-<name>` aliases that `~/.ssh/config` already includes
from `~/.lima/*/ssh.config`. Because Lima regenerates that file on every start,
the alias keeps working when the VM's SSH port changes.

- **Connecting.** In the Claude desktop app, add an SSH connection to host
  `lima-<name>` (for example `lima-scratch`) and point it at a directory under
  `~/code`.
- **What the VM must provide:** a working SSH login, plus the toolchain on
  PATH for non-interactive commands. `mise.sh` handles PATH; Lima handles SSH.
- **Git push from the VM.** See *Git and SSH keys* below.
- **Isolation.** Code lives in the VM (D1), so anything Claude runs over SSH
  can only change the VM, never the host.

## Git and SSH keys

- **Keys stay on the Mac.** `ssh.forwardAgent: true` in `layer-os.yaml`
  forwards the Mac's ssh-agent. Git in the VM signs with keys held there.
  `~/.ssh` is deliberately not mounted, so nothing in the VM, Claude included,
  can read a private key. Requirement: the key must be loaded in the Mac's
  agent (`ssh-add --apple-use-keychain ~/.ssh/id_rsa`, plus `AddKeysToAgent yes`
  and `UseKeychain yes` in `~/.ssh/config` so it survives a reboot).
- **gitconfig comes from the host.** `layer-base.yaml` mounts the
  `gitConfigDir` param (default `~/.config/git`, git's XDG location) read-only
  at `/mnt/host/git`. Override it per VM with `--param gitConfigDir=…`. Lima's
  global `override.yaml` changes the param value but not the already-expanded
  mount path, so it can't be used for this. `features/git.sh` writes a VM `~/.gitconfig`
  that `[include]`s the first config it finds there (`config` or `.gitconfig`),
  then overrides values that only make sense on the host (`core.editor`) and
  points `excludesfile` and `commit.template` at files in the mount. Edits on the Mac apply at once. `bat`
  comes from `mise/base.toml` because the host config uses it as the pager.
- **GitHub host keys** are written to `known_hosts` from
  `api.github.com/meta` (fetched over TLS), so the first push doesn't stop at a
  prompt.

## Where code lives (decision D1)

| Option | Pros | Cons |
|---|---|---|
| **A. Code lives in the VM** (`~/code`, cloned there) | Strong isolation, fast native FS, a natural fit for letting Claude run freely | Code goes away with the VM unless pushed. Rebuild means re-clone |
| B. Mount `~/code` writable | Same files as the host, survives rebuilds | Claude in the VM can change host files. virtiofs is slower for node_modules and similar |
| C. Per-VM extra disk (`limactl disk create`) at `~/code` | Survives rebuilds, still isolated | One more thing to manage |

**Decided: A.** Code is cloned into `~/code` inside the VM with ghq
(`~/code/<host>/<owner>/<repo>`; the `codeDir` param). C (persistent disk)
and an opt-in `--mount` can come later if rebuilds start to hurt.

## Other decisions to make

- **D2 Base distro. Decided: Ubuntu 24.04** (`template:ubuntu-24.04`), pinned
  rather than `ubuntu-lts` so rebuilds stay reproducible.
- **D3 Shell config portability.** For when dotfiles return: shell configs
  often assume macOS (`/opt/homebrew`, `pbcopy`). They need
  `[[ $OSTYPE == darwin* ]]` guards to work in both places.
- **D4 Container runtime. Decided: Lima's rootless containerd + nerdctl**
  (`containerd.user: true`), with `docker` symlinked to `nerdctl` by
  `features/nerdctl.sh`. Lima installs and upgrades it, so there's no install
  script to maintain. Known gaps: tools that need a real Docker daemon socket
  (for example testcontainers), and kind's nerdctl provider, which is
  experimental. To switch to Docker Engine, add a system feature script and set
  `containerd.user: false`. If you do, set `SocketUser` on `docker.socket`:
  a docker group added during first boot doesn't reach Lima's already-open SSH
  session.
- **D5 vmType.** `vz` with `rosetta` enabled lets you run x86 binaries when
  needed. It's the default on Apple Silicon.

## Environments

The base layer is deliberately broad: shell tools, Go, Python, containers,
Kubernetes and Terraform tooling in every VM. That keeps a single manifest to
maintain. Environment templates exist for what really differs: resources, or
heavy extras most VMs don't need.

| Name | Adds on top of base |
|---|---|
| `scratch` | nothing: a throwaway sandbox |
| (future) `node` | node LTS, pnpm |
| (future) `k8s-big` | more CPU/memory for kind clusters |

## Milestones

1. ✅ `layer-os.yaml` + `layer-base.yaml` + the `apt-common`, `mise`, `git` and
   `nerdctl`/`shell` features, plus the global editorconfig. `scratch` boots, and `ssh lima-scratch` has every tool.
2. A second environment, to exercise per-environment mise TOML files.
3. `bin/limabean` with `new/rebuild/lint`.
4. Dotfiles (`features/dotfiles.sh`). Code-location option C (persistent
   disk) if rebuilds become painful.
5. A smoke test: `limabean lint` plus a script that rebuilds `scratch` and
   checks the probes.
