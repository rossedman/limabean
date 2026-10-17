# Design notes

The decisions behind limabean, with the trade-offs considered. For usage, see
the [README](../README.md) and the other pages in this folder.

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
   [getting started](getting-started.md).

## Non-goals

- Supporting other operating systems or hypervisors (macOS host, Lima only).
- Managing the user's dotfiles. limabean reads host config read-only and
  never owns it.
- Production or CI images.

## Host prerequisites

These are documented for users in [getting started](getting-started.md):

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
  layer-os.yaml       # layer 1: Ubuntu 24.04, CPU/mem/disk, ssh agent forwarding, zsh
  layer-base.yaml     # layer 2: base: layer-os + apt + Docker + mise tools + git + zsh + editorconfig
  scratch.yaml        # layer 3: base: layer-base, nothing else (throwaway sandbox)
  go.yaml             #          base: layer-base + mise/go.toml
  python.yaml  k8s.yaml  ...
  features/           # idempotent shell scripts, one per capability
    zsh-install.sh  apt-common.sh  mise.sh  completions.sh  git.sh  docker.sh  shell.sh
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
- **Shell.** zsh is the login shell from the moment the account exists:
  `user.shell` in `layer-os.yaml`, with zsh installed by a `mode: dependency`
  script before the first login. Changing the shell later (`usermod`) doesn't
  work. Lima's persistent SSH connection keeps the shell and groups it saw at
  login, and closing that connection from inside the guest breaks Lima's
  readiness checks. `files/zshrc` holds
  plain zsh config with a `vcs_info` prompt. There's deliberately no prompt
  framework or plugin manager: two apt plugins (autosuggestions, syntax
  highlighting) and fzf's built-in keybindings. Managed files are rewritten on
  every boot; personal additions go in `~/.zshrc.local`.
- **Completions** are generated once per boot by `features/completions.sh`
  into a single fpath directory, not with `eval "$(tool completion zsh)"` in
  zshrc. That keeps shell startup fast. Tools without zsh output (aws,
  terraform, go through gocomplete) use `bashcompinit`.

## Dotfiles: deferred

The first version covers provisioning only. When dotfiles come back, the plan is
to mount a dotfiles directory (a param, like `gitConfigDir`) read-only and
apply a Linux-safe subset in a
`features/dotfiles.sh` script. This needs D3 (shell config portability) to be
settled first.

## Wrapper CLI: `bin/limabean`

A single bash script (bash 3.2-compatible, so macOS's stock `/bin/bash` runs
it) that wraps `limactl`. [cli.md](cli.md) has the command reference. Design points:

- **Config file instead of Lima params.** `~/.config/limabean/config` (sourced
  shell: `GIT_CONFIG_DIR`, `CODE_DIR`, `CPUS`, `MEMORY`) is passed to
  `limactl start` as `--param`/`--cpus`/`--memory`. It replaces Lima's
  `override.yaml`, which can't set mount paths.
- **`doctor` turns silent failures into explicit ones**: a missing or
  misplaced `Include` (inside a `Host`/`Match` block it doesn't apply), an
  empty ssh-agent, and a missing git config.
- **`rm`/`rebuild` guard unsaved work.** Over SSH, every git repo under
  `CODE_DIR` is checked for uncommitted changes, stashes and commits on no
  remote (`git log --branches --not --remotes`). `--force` skips the check.
- **The environment is remembered** in `~/.local/state/limabean/<name>.env`, so
  `rebuild` recreates from the same template. `ls` shows only VMs built from
  these templates (their `lima.yaml` references `/etc/limabean/mise/`).
- **Claude Desktop registration.** `new` adds an entry to `sshConfigs` in
  `~/.claude/settings.json` (documented: users may set it in their own settings
  file) with `sshHost: lima-<name>`. `rm` removes it (also for a VM already
  deleted with `limactl delete`), `ls` and `new` prune entries for VMs that no
  longer exist, and `claude sync` reconciles. Only ids prefixed `limabean-` are touched. Edits use `jq`, are
  skipped if the file isn't valid JSON, and are written in place so file modes
  and symlinks survive. Desktop installs Claude Code on the VM itself, so
  nothing Claude-specific is provisioned.
- **Templates are found next to the script** (symlinks resolved), so it can be
  symlinked onto PATH from a checkout.

`rebuild` is the main maintenance path: change a template, rebuild, done.

## Claude over SSH

Claude is not provisioned into the VMs. Claude runs from the Mac and connects
over SSH, using the `lima-<name>` aliases that `~/.ssh/config` already includes
from `~/.lima/*/ssh.config`. Because Lima regenerates that file on every start,
the alias keeps working when the VM's SSH port changes.

- **Connecting.** `limabean new` registers the VM in Claude Desktop (see
  *Wrapper CLI*). It appears as `<name> (limabean)` under SSH in the
  environment dropdown, and connects to the `lima-<name>` alias.
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
- **D4 Container runtime. Decided: Docker Engine** (`features/docker.sh`,
  from Docker's apt repository, with the buildx and compose plugins), and
  Lima's containerd/nerdctl turned off (`containerd.user: false`). An earlier
  version used Lima's rootless nerdctl with `docker` symlinked to it. That was
  less to maintain, but it couldn't run the full Docker CLI and plugin
  ecosystem, custom BuildKit frontends, or kind's default provider. The VM user
  owns `docker.socket` through a systemd `SocketUser` drop-in, because a docker
  group added during first boot doesn't reach Lima's already-open SSH session.
- **D6 Package updates. Decided: full upgrade on first boot.**
  `features/apt-upgrade.sh` runs `apt-get full-upgrade` once, systemd
  included, and runs `systemctl daemon-reexec` so PID 1 uses the new systemd.
  `needrestart` is suspended during provisioning. When
  `/var/run/reboot-required` exists (a new kernel, say), `limabean new`
  restarts the VM before handing it over. Later boots skip the upgrade, so
  restarts stay fast; `rebuild` is how a VM gets newer packages. Packages held
  back by Ubuntu's phased updates stay held, which is intended.
- **D7 GitHub token. Decided: a scoped token in the macOS Keychain, copied in
  by limabean.** Copying the host's `gh auth token` would hand every VM the
  host's broadest credential, typically `repo`, `workflow` and `read:org`. It
  would also pick the wrong host when `gh` defaults to GitHub Enterprise, and
  go stale when the host re-authenticates. Per-VM `gh auth login` is
  repeated after every rebuild. So `limabean github login` stores one
  fine-grained token in the Keychain and writes it to
  `~/.config/limabean/github-token` (mode 600) in each VM, where `env.sh`
  exports `GITHUB_TOKEN`. It never goes into a template, because those are
  plain text under `~/.lima`. It's passed on stdin, never in argv. git uses
  `gh auth git-credential` for `https://github.com`. A token served on demand
  from the Mac over a forwarded socket was considered and rejected: any
  process in the VM could still request it, so it adds moving parts without
  real protection.
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
   `docker`/`shell` features, plus the global editorconfig. `scratch` boots, and `ssh lima-scratch` has every tool.
2. A second environment, to exercise per-environment mise TOML files.
3. ✅ `bin/limabean`: `init`, `doctor`, `new`, `ls`, `ssh`, `get`, `stop`,
   `rebuild` and `rm`, with the unsaved-work guard.
4. Dotfiles (`features/dotfiles.sh`). Code-location option C (persistent
   disk) if rebuilds become painful.
5. `limabean bump` (rewrite pinned versions in `base.toml`), CI (`limactl
   tmpl validate` + shellcheck), and a persistent `~/code` disk so rebuilds
   keep repositories.
