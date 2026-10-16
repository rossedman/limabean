# limabean — spec (draft)

Reproducible Lima VMs that act as personal dev environments: my tools, my
dotfiles, one per purpose. Claude connects to them over SSH and works inside
them.

**Main goal: easy to configure and maintain.** If a decision makes it harder to
add an environment or change a tool once for all of them, it's the wrong
decision.

## Goals

1. Defining a new environment means writing about 10 lines of YAML.
2. Each tool, dotfile or setting is defined in exactly one place.
3. VMs can be thrown away. Rebuilding one is one command and nothing is lost.
4. Claude, running on the Mac, can reach any environment over SSH by a stable
   host name.
5. It works with Lima as it ships (Lima 2.2.1, installed). No extra layers
   such as Nix or Ansible unless they clearly earn their place.

## Non-goals

- Supporting other operating systems or hypervisors (macOS host, Lima only).
- Replacing `~/.dotfiles`. limabean reads it and never owns it.
- Production or CI images.

## What already exists

| Thing | State | Implication |
|---|---|---|
| Lima | 2.2.1 via Homebrew | `base:` template composition and `param:` are available |
| `~/.ssh/config` | already has `Include ~/.lima/*/ssh.config` | every VM is reachable as `ssh lima-<name>` with no extra work |
| Dotfiles | `~/.dotfiles`, GNU Stow packages (`zsh`, `git`, `gh`, `fzf`, `ripgrep`, `claude`, …) | stow a Linux-safe subset inside the VM |
| `Brewfile` | macOS-oriented (casks, raycast, vscode) | don't reuse it for Linux. Use a separate tool manifest |

## Architecture: layered templates

Lima templates can inherit from other templates through `base:`. Use that to
build three layers:

```
templates/
  layer-os.yaml       # layer 1: Ubuntu 24.04, CPU/mem/disk, ssh agent forwarding, no host mounts
  layer-base.yaml     # layer 2: base: layer-os + apt basics + mise
  scratch.yaml        # layer 3: base: layer-base, nothing else (throwaway sandbox)
  go.yaml             #          base: layer-base + mise/go.toml
  python.yaml  k8s.yaml  ...
  features/           # idempotent shell scripts, one per capability
    apt-common.sh  mise.sh
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
- **PATH for SSH commands.** `mise.sh` puts `~/.local/bin` and the mise shims at
  the top of `~/.bashrc`, above Ubuntu's non-interactive guard, so
  commands run over SSH (`ssh lima-<name> go test ./...`) see the tools without
  a login shell.

## Dotfiles: deferred

The first version covers provisioning only. When dotfiles come back, the plan is
to mount `~/.dotfiles` read-only and `stow` a Linux-safe subset in a
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
  `~/src`.
- **What the VM must provide:** a working SSH login, plus the toolchain on
  PATH for non-interactive commands. `mise.sh` handles PATH; Lima handles SSH.
- **Git push from the VM.** `ssh.forwardAgent: true` in `layer-os.yaml`. Keys
  stay on the Mac.
- **Isolation.** Code lives in the VM (D1), so anything Claude runs over SSH
  can only change the VM, never the host.

## Where code lives (decision D1)

| Option | Pros | Cons |
|---|---|---|
| **A. Code lives in the VM** (`~/src`, cloned there) | Strong isolation, fast native FS, a natural fit for letting Claude run freely | Code goes away with the VM unless pushed. Rebuild means re-clone |
| B. Mount `~/code` writable | Same files as the host, survives rebuilds | Claude in the VM can change host files. virtiofs is slower for node_modules and similar |
| C. Per-VM extra disk (`limactl disk create`) at `~/src` | Survives rebuilds, still isolated | One more thing to manage |

**Decided: A.** Code is cloned into `~/src` inside the VM. C (persistent disk)
and an opt-in `--mount` can come later if rebuilds start to hurt.

## Other decisions to make

- **D2 Base distro. Decided: Ubuntu 24.04** (`template:ubuntu-24.04`), pinned
  rather than `ubuntu-lts` so rebuilds stay reproducible.
- **D3 Shell config portability.** Check `~/.dotfiles/zsh` for macOS-only
  assumptions (`/opt/homebrew`, `pbcopy`). Guard them with
  `[[ $OSTYPE == darwin* ]]` in the dotfiles repo so one config works on both.
- **D4 Container runtime.** Docker or nerdctl inside environments that need it
  (k8s). Make it a feature script, not a separate template family.
- **D5 vmType.** `vz` with `rosetta` enabled lets you run x86 binaries when
  needed. It's the default on Apple Silicon.

## Starter environments

| Name | Adds on top of base |
|---|---|
| `scratch` | nothing: a throwaway sandbox for Claude |
| `go` | go, gopls, golangci-lint |
| `python` | uv, python 3.13, ruff |
| `node` | node LTS, pnpm |
| `k8s` | docker, kind, kubectl, helm, k9s |

## Milestones

1. `layer-os.yaml` + `layer-base.yaml` + the `apt-common` and `mise` features.
   `scratch` boots, and `ssh lima-scratch rg --version` works.
2. `go` and `python` environments. Confirm the `base:` merge behaviour.
3. `bin/limabean` with `new/rebuild/lint`.
4. Dotfiles (`features/dotfiles.sh`). Code-location option C (persistent
   disk) if rebuilds become painful.
5. A smoke test: `limabean lint` plus a script that rebuilds `scratch` and
   checks the probes.
