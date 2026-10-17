# Design notes

The decisions behind limabean, with the trade-offs considered. For usage, see
the [home page](index.md) and the other pages of these docs.

Reproducible Lima VMs that act as personal dev environments, one per purpose,
usable by anyone on macOS. Claude, or any SSH client, connects to them over SSH
and works inside them.

**Main goal: easy to configure and maintain.** If a decision makes it harder to
add an environment or change a tool once for all of them, it's the wrong
decision.

## Goals

1. Defining a new environment means writing about 10 lines of YAML.
2. Each tool, dotfile or setting is defined in exactly one place.
3. VMs can be thrown away. Rebuilding one is one command and nothing is lost:
   `~/code` is on its own disk.
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
  layer-os.yaml       # layer 1: Ubuntu 26.04 LTS, CPU/mem/disk, ssh agent forwarding, zsh
  layer-base.yaml     # layer 2: base: layer-os + apt + Docker + mise tools + git + zsh + editorconfig
  scratch.yaml        # layer 3: base: layer-base, nothing else (throwaway sandbox)
  go.yaml             #          base: layer-base + mise/go.toml
  python.yaml  k8s.yaml  ...
  features/           # idempotent shell scripts, one per capability
    zsh-install.sh  apt-common.sh  mise.sh  completions.sh  git.sh  docker.sh  shell.sh
  files/              # static files written into the VM (zshrc, env.sh, editorconfig)
  mise/               # tool versions; base.toml + one per environment
    base.toml  go.toml  python.toml
cmd/, internal/         the limabean CLI (Go; see go-cli.md)
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

## The CLI

`limabean` started as a bash script and is now a Go binary built with Cobra
and Viper. It drives `limactl`, embeds the templates, and adds secrets with a
creation form. Its design is in [go-cli.md](go-cli.md); the behaviors below
carried over unchanged:

- **Config file** (`~/.config/limabean/config`, `KEY="value"`) instead of
  Lima's `override.yaml`, which can't set mount paths.
- **`doctor` turns silent failures into explicit ones**: a missing or
  misplaced `Include`, an empty ssh-agent, a missing git config, broken
  secret references.
- **`rm` guards unsaved work** in `~/code` (uncommitted changes, stashes,
  commits on no remote), and so does `rebuild` for a VM without a code disk.
  `--force` skips the check.
- **Claude Desktop registration** in `~/.claude/settings.json` `sshConfigs`,
  touching only `limabean-` ids, keeping other keys and their order, leaving
  invalid JSON alone, and writing in place so symlinks survive.
- **The environment is remembered** in `~/.local/state/limabean/<name>.env`,
  so `rebuild` recreates from the same template.

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

**Decided: A, then C.** Code is cloned into `~/code` inside the VM with ghq
(`~/code/<host>/<owner>/<repo>`; the `codeDir` param), and since re-cloning
after every rebuild hurt, it lives on a per-VM Lima disk:

- `limabean new` creates `code-<6 hex of sha256(vm)>` (`CODE_DISK` GiB,
  raw and sparse) and attaches it with `--set .additionalDisks` and the
  `codeDisk` param. `rebuild` reattaches the same disk; `rm` deletes it.
- Lima formats a disk on any boot that can't find the label
  `lima-<name>`, and ext4 labels hold 16 bytes. A name over 11 characters
  would be truncated and the disk erased on every boot, hence the hash.
- Lima mounts the disk at `/mnt/lima-<name>`, which isn't configurable, so
  `features/code-disk.sh` (system, every boot) bind-mounts it over
  `codeDir` before the user scripts run.
- `limactl delete --force` kills a running VM, so `rebuild` stops it first
  and pending writes reach the disk.

An opt-in `--mount` of a host directory (B) is still open.

## Other decisions to make

- **D2 Base distro. Decided: Ubuntu 26.04 LTS** (`template:_images/ubuntu-26.04`),
  pinned rather than `ubuntu-lts` so rebuilds stay reproducible. Moved from
  24.04, whose systemd 255 only gets patches and is too old for Varlink tooling
  such as vlx (needs v257+). 26.04 ships systemd 259 with `varlinkctl` and the
  `io.systemd.*` Varlink interfaces, and Docker's apt repository supports it.
  Upgrading systemd within 24.04 isn't possible.
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
- **D8 Interface. Superseded by D11.** The bash CLI used gum, with a plain
  fallback. The Go CLI keeps the same behavior (spinners, pickers, forms,
  confirmations, tables; plain text and no prompts without a terminal or with
  `--plain`) using Charm's Go libraries, so gum is no longer needed.
- **D9 GitHub push transport. Decided: https with the token when one is
  installed, SSH otherwise.** Claude Desktop's helper in the VM runs without
  `SSH_AUTH_SOCK`, so SSH pushes can't work in Claude sessions.
  `limabean-github-git` writes `~/.config/limabean/github.gitconfig`: with a
  token, `insteadOf` rules send GitHub SSH remotes over https; without one,
  `pushInsteadOf` sends https pushes over SSH.
- **D10 AWS. Decided: mount `~/.aws` read-only when it exists.** A writable
  mount would let the VM edit `~/.aws/config`, whose `credential_process` runs
  commands on the Mac. Read-only means SSO logins happen on the Mac. limabean
  adds the mount at creation (`--set .mounts += …`) only when the directory
  exists, so the templates stay host-independent.
- **D11 CLI language. Decided: Go, driving `limactl`.** Bash had reached its
  limits: bash 3.2 compatibility, quoting, a SIGPIPE false negative, and no
  good way to build a secrets form. The Go CLI uses Cobra and Viper, one
  command per file, with every command's help enforced by a test. It uses
  huh and lipgloss for the interface, embeds the templates, and ships as one
  universal binary built with GoReleaser. It doesn't import Lima's Go
  packages, which make no promise of a stable API; `limactl` and
  `limactl list --format json` are the boundary.
- **D12 Secrets. Decided: references on the Mac, values only in VMs.**
  `secrets.env` and `secrets.<vm>.env` hold `NAME=reference` lines
  (`keychain:`, `op://`, `env:`, `cmd:` or a plain value). `secrets sync`
  resolves them on the Mac and writes `export` lines (mode 600) into each VM.
  The GitHub token is one of these secrets. Values never reach argv or a Lima
  template.
- **D13 Snapshots. Decided: APFS clones of the disk files.** Lima 2.2's
  `vz` driver returns "unimplemented" for `limactl snapshot`, which only
  works with QEMU. With the VM stopped, limabean clones `disk` (or
  `diffdisk`), `vz-efi` and the code disk's `datadisk` with `cp -c` into
  `<instance>/limabean-snapshots/<tag>/`. Restores clone each file beside
  its original and rename it into place, so a failure never leaves a
  half-written disk. Live snapshots (memory included) would need
  Virtualization.framework's save/restore, which Lima doesn't expose.
- **D15 Network allowlists. Decided: dnsmasq and nftables in the VM, plus
  an opt-in lock.** dnsmasq answers only allowed domains (`address=/#/`
  makes the rest NXDOMAIN, which also closes DNS as a way out) and adds the
  addresses it returns to nftables sets (`nftset=`); an `inet limabean`
  table drops all other output and forwarded traffic, so containers are
  covered, and only dnsmasq's user may reach the upstream DNS server.
  The policy file is written on the Mac and mounted read-only, so the VM
  can't edit it; `limabean-network` applies it at boot (after the mount)
  and on `systemctl start limabean-network`. Inside the VM this isn't a
  boundary on its own: Lima gives the user passwordless sudo, and the
  Docker socket is root. So `locked` replaces cloud-init's sudoers entry
  with two commands (Lima's host agent runs `sudo cat` on its param file at
  every start; re-applying the policy only reads the Mac's copy), drops the
  sudo/docker groups and the socket ownership, and is re-applied every boot
  because cloud-init and docker.sh would restore them. Enforcing on the
  Mac (a filtering proxy plus pf rules) would hold against root in the VM
  too, but needs admin rights on the Mac; it's the next step if the lock
  isn't enough.
- **D14 MCP server. Decided: `limabean mcp`, stdio, VM-scoped tools.**
  Built on the official Go SDK. Each tool runs limabean's own command
  in-process, with a plain UI capturing its output and `ssh`/`get`'s
  terminal handoff disabled (stdout carries the protocol), so every guard
  applies. Threat model: the client is an agent that may have read hostile
  content (a README, an issue, a web page) and been steered by it. So:
  - nothing runs on the Mac: `run_in_vm` and `clone_repo` go over ssh
    into a running VM, and secrets aren't exposed at all, since a `cmd:`
    reference is a command on the Mac;
  - names and snapshot tags are validated before reaching the command line,
    where `--force` would be a flag, and a repository can't start with `-`;
  - tools that destroy work exist only with `--allow-destructive`, carry
    `_meta["anthropic/requiresUserInteraction"]` (Claude Code then asks before
    every call, in any permission mode) and `destructiveHint` for other
    clients, and `delete_vm` keeps rm's unsaved-work check;
  - one limabean command runs at a time; `run_in_vm` has a timeout (10
    minutes by default, an hour at most) and returns the last 100 KiB.
  What a VM can reach (its secrets, GitHub with its token, the network)
  an agent in it can reach too; scoping that is per-VM secrets' job.
- **D5 vmType. Decided: `vz`, with Rosetta.** `layer-os.yaml` sets
  `vmOpts.vz.rosetta` with `binfmt: true`, so x86_64 binaries and
  `linux/amd64` containers run on Apple Silicon without a prefix (Lima
  registers it with the `F` flag, which Docker needs). On Intel Macs Lima
  logs that Rosetta is unavailable and boots normally. QEMU user-mode
  emulation was the alternative: slower, and one more package to install.

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
3. ✅ The CLI: `init`, `doctor`, `new`, `ls`, `ssh`, `get`, `stop`,
   `rebuild` and `rm`, with the unsaved-work guard.
4. Dotfiles (`features/dotfiles.sh`). ✅ Code-location option C (persistent
   disk).
5. ✅ `limabean bump` (rewrite pinned versions in `base.toml`, monthly in
   `bump.yml`), ✅ CI (`limactl
   tmpl validate` + shellcheck), and ✅ a persistent `~/code` disk so
   rebuilds keep repositories.
