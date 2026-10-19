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
  features/           # idempotent shell scripts, one per capability
    zsh-install.sh  apt-common.sh  mise.sh  completions.sh  git.sh  docker.sh  shell.sh
  files/              # static files written into the VM (zshrc, env.sh, editorconfig)
  packs/              # tool packs: <pack>.toml (pinned mise tools) + optional <pack>.apt
    core.toml  go.toml  node.toml  rust.toml  ...
cmd/, internal/         the limabean CLI (Go; see go-cli.md)
```

Everything sits under `templates/` because Lima rejects relative paths that
contain `..` and base templates whose file names start with `_`.

Languages and tools come in [packs](packs.md), chosen per VM rather than per
environment. `limabean new` adds each pack beyond `core` with
`--set '.provision += [{"mode": "data", "path": "/etc/limabean/mise/50-<pack>.toml", "content": …}]'`,
so an environment template stays tiny and any environment can have any
pack. The base layer carries everything that is shared:

```yaml
# templates/layer-base.yaml
base:
- ./layer-os.yaml
provision:
- mode: system
  file: ./features/apt-common.sh
- mode: data
  path: /etc/limabean/mise/00-core.toml
  file: ./packs/core.toml
- mode: system
  file: ./features/packs-apt.sh     # apt packages of the VM's packs, /etc/limabean/apt/*.list
- mode: user
  file: ./features/mise.sh          # installs mise, links /etc/limabean/mise/*.toml, mise install
```

### Why this layout

- **Feature scripts plus thin YAML.** Logic lives in plain shell scripts. The
  YAML only says which features an environment gets. Adding a tool to every
  environment means editing one file.
- **mise for toolchains (decided).** Each pack is a declarative
  `packs/<pack>.toml` with pinned versions, so upgrading Go is a one-line diff.
  apt covers system packages: the shared ones, and a pack's `.apt` list.
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
  comes from `packs/core.toml` because the host config uses it as the pager.
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
- **D16 Fast creation. Decided: clone a provisioned image.** Provisioning
  (full upgrade, Docker, every pack's tools) took minutes per VM; booting
  takes seconds. An image is a VM provisioned exactly like any other but
  with an empty network policy, no code disk, no AWS mount and no secrets,
  then cleaned (`apt-get clean`, an empty `/etc/machine-id` so each copy
  gets its own, as cloud images do) and stopped. A new VM is
  `limactl clone` of it, with `--set` replacing the per-VM params and
  mounts (git config, network policy, code dir), adding its code disk and
  AWS mount, and setting its size; then a normal boot, whose provisioning
  scripts find everything installed. Lima regenerates the cloud-init
  instance ID, SSH host keys and MAC address per instance, and networking
  uses `dhcp-identifier: mac`, so copies don't collide.
  - The key is a hash of the image format, every file under the templates
    directory (user templates and packs included), the environment and the
    packs. Anything that would change a VM changes the key, and outdated
    images are pruned rather than patched.
  - No image yet means the old path plus a detached
    `images build --if-needed`, so the first VM is no slower than before.
    Images older than a week are used and replaced in the background,
    bounding how stale a copy's packages are.
  - flock guards: one build per key (`.build`), and copies hold `.use`
    shared while the builder takes it exclusively to delete the image it
    replaced.
  - Considered: snapshotting the first real VM before its secrets were
    installed (saves one provision, but risks carrying VM-specific state
    into the image), and Lima's own image caching (only the download).
- **D17 Guardrails in VMs. Superseded by D18: plugins come from a
  project's `.limabean.yaml`, so no plugin is in every VM.** Originally:
  declare the plugin in the VM's user settings. `features/guardrails.sh` adds the `rossedman/guardrails`
  marketplace and `guardrails@guardrails` to `~/.claude/settings.json` on
  every boot, only where they're missing, so `false` sticks. Claude Code
  fetches a plugin enabled in user settings when a session starts, with no
  trust prompt (project settings have one), and Claude Desktop's SSH copy
  and the `claude` pack's both read that file. The packs carry the
  formatters and linters guardrails runs, so it doesn't download them.
  - Considered: `claude plugin install` during provisioning (needs Claude
    Code installed and the network at that moment; Claude Desktop's copy
    arrives only on first connect).
- **D18 Claude plugins from a project file. Decided: user settings
  written in the VM by `up`.** `.limabean.yaml`'s `claude.marketplaces` is a
  list of entries, each a marketplace (GitHub `owner/repo` or an https git
  URL) and the `plugin@marketplace`s to turn on from it; the name after the
  `@` is the settings key, so one entry's plugins must agree on it.
  `project.Load` checks every entry against a strict pattern, the plan lists
  them, and `up` sends them to a python script in the VM as base64 JSON on
  stdin, so the file's text never reaches a command line on either side.
  The script merges them into `~/.claude/settings.json`, and Claude Code
  fetches them at session start (as in D17), under the VM's network policy.
  - Skills aren't listed separately: plugins ship them, along with agents,
    hooks and MCP servers.
  - Considered: `claude plugin install` (needs the claude pack, and misses
    Claude Desktop's copy), and leaving it to a repository's own
    `.claude/settings.json` (needs a trusted folder; nothing for headless
    runs or other folders).
- **D19 Recipes. Decided: embedded starter project files, tested by
  bringing each one up.** `recipes/*.yaml` are `.limabean.yaml` files with
  comments, embedded by a small `recipes` package so the binary carries
  them; `limabean recipes` lists them (the first comment line is the
  description) and writes one into the current directory without
  replacing a project file unless forced. They live outside `templates/`
  so editing one doesn't change the templates hash and outdate every image.
  `TestRecipesBringVMsUp` runs `up` on each, so a recipe naming a missing
  pack or preset fails CI, and the docs include the files themselves.
- **D20 Packs. Decided: none by default; a project lists all of its own.**
  VMs used to get Go, Python, Node, Kubernetes, infra, container and
  database packs unless told otherwise, and most recipes were lists of
  packs to remove. Now `PACKS` is empty: a VM has `core`, Docker and
  Python, and gets what `new`'s form picks (`PACKS` pre-checked), what
  `--with` names, or what its `.limabean.yaml` lists. A project's packs
  are exact, ignoring `PACKS`, so `up` builds the same VM for everyone;
  `without` still loads, with a warning, so older files work. Configs
  written by `init` before keep the old list in `PACKS`, so existing users
  see no change until they clear it. Smaller default images build faster
  too.
- **D21 up and down. Decided: up makes an existing VM match its file;
  down stops it.** Like `docker compose`, the file is the source of truth:
  `up` on an existing VM compares the file with the VM as it is (saved
  packs and environment, Lima's size, the saved network policy, the VM's
  secrets file), not with a stored copy, so it also works for VMs made
  before it. It lists the differences and asks (`--yes` for scripts), then
  rebuilds for packs or environment (keeping the `~/code` disk; refused
  without one while work is unpushed), stops, edits and starts for size,
  adds secrets and Claude plugins, and applies a tighter network. It never
  widens a network or lifts a lock: those stay with the user's own
  `limabean network` commands, which `up` prints. Setup runs once, or
  again with `--setup`. `down` only stops the VM, keeping everything,
  rather than deleting it as Compose does: a VM holds more than a
  container, and `rm` already deletes with its checks.
- **D22 Two verbs. Decided: up and down make, start and stop every VM.**
  There were four ways to bring a VM up (`new`/`create`, `start`, `up`, and
  the commands that start one on the way) and two to stop one. Now `up`
  works from a project file when there is one and the name (if any) is
  the project's, and otherwise starts the named VM or creates it with
  `new`'s form and flags. Flags that only mean something when creating
  are refused for an existing VM or a project, rather than ignored.
  `down` stops the project's VM or the named one, and takes `stop`'s
  `--idle`. `new`, `create`, `start` and `stop` stay as hidden commands:
  scripts keep working, and autostop agents installed earlier run
  `stop --idle`. `up` applies its size flags itself, since a setting can
  bind only one flag and `new` keeps its binding.
- **D23 start and logs. Decided: start runs detached after every up, and
  is approved like a change.** `setup` runs once; what should be running
  whenever the VM is (services, a dev server) goes in `start`, which `up`
  launches with `setsid nohup` so it outlives the ssh session, keeping a
  pid file and skipping the launch while the last one still runs (a
  zombie doesn't count, for inits that don't reap). Its output and
  setup's (through `tee`, keeping the exit status) go to
  `~/.local/state/limabean/` in the VM, which `limabean logs` reads, along
  with cloud-init's log (needs sudo, so not in a locked VM) and Lima's
  host log on the Mac. Because start runs again on every `up` and the file
  can change with a `git pull`, the last approved commands are kept on the
  Mac; different ones are a change `up` shows and asks about, and until
  then neither old nor new run. `up --dry-run` prints the plan or the
  changes and stops.
- **D24 vars, ps and project tools over MCP. Decided: the project
  file stays the one description, for people and for Claude.**
  `vars:` writes plain variables to `~/.config/limabean/project.env`,
  which `env.sh` sources before `secrets.env` so secrets win; names the
  shell or linker reads first (`PATH`, `LD_*`, …) and names that are also
  secrets are refused. The applied set is kept on the Mac so changes are
  shown and applied live. `ps` combines the VM's status, start, ports and
  `up`'s pending changes. Over MCP, `project_plan` (read-only) and
  `project_up` (with `requiresUserInteraction`, so the user approves each
  call after seeing the plan) run `up` itself, so every rule holds:
  networks only tighten, start changes need approval, and a rebuild needs
  `--allow-destructive`. MCP input picks only a directory, whose own
  regular `.limabean.yaml` is the one file read (no parents, no symlinks,
  so a call can't turn limabean into a file reader), and the origin comes
  from parsing `.git/config`, so git never runs on the Mac for a tool
  call. Setup, which needs a terminal in the CLI, runs without one over
  MCP and its output goes into the reply.
- **D25 Following [cli.style](https://cli.style/). Decided: adopt its
  rules on streams, exit codes, flag names and discoverability, keep
  limabean's verbs.** stdout carries only data, and status goes to stderr.
  Exit codes are fixed (2 usage, 4 not found, 5 in the way, 130
  cancelled), and `--json` failures are JSON. `help --json` and
  `version --json` describe the CLI to scripts and agents. Flags are named
  for their outcome (`--preview`, `--yes`, `--overwrite`), with the old
  names kept and hidden. `config` shows where each setting comes from.
  limabean's own files are written atomically, and a credential typed on
  the command line gets a warning. Not adopted: noun-verb grammar (`up` and
  `down` stay, D22); dropping success lines (they moved to stderr, and
  `-q` hides them); macOS `~/Library` paths (`~/.config` and XDG stay); and
  editing `~/.ssh/config` only on request (`init` asks first, and
  limabean can't connect without the `Include`). The hidden `new`, `start`
  and `stop` print a note naming what replaced them. No removal date is
  set.
- **D26 Project tools and a schema. Decided: a project file names its own
  tools, and editors check it.** `tools:` (mise, `tool: version`) and
  `apt:` cover what no pack has, without a pack on every Mac. They're
  written into the VM next to the packs' lists at creation
  (`90-project.toml`, `_project.list`), so the same boot scripts install
  them and VMs copied from an image get them; a change rebuilds, as a pack
  change does, since a locked VM has no sudo to install them live. Only
  names and versions are accepted, so the file can't break out of the
  TOML or the apt list, and nothing runs on the Mac. The JSON Schema
  published with the docs is generated from Go, using `project.Load`'s own
  patterns; tests fail if a key is missing from it, if the published copy
  is stale, or if a recipe doesn't validate.
- **D27 Writing and checking project files. Decided: `init --project`
  guesses, and `doctor` checks what only the Mac knows.** `init --project`
  reads manifests, lockfiles, framework hints, compose files, tool-version
  files and `.env.example` in the repository's root and the directories
  just below it, and writes a commented draft that must pass the same
  checks as `up` before it's written. It's part of `init` rather than a new
  verb, and explicit (`--project`) rather than switched on by running
  `init` in a checkout, so first-time setup never writes into a
  repository by surprise. It only reads files, never runs repository code,
  doesn't follow symlinks, and skips directories whose names would change
  what `setup`'s `cd` means. `doctor` gains a Project section for what the
  schema can't know: whether the packs, presets and secrets exist on this
  Mac, and whether the VM matches the file.
- **D28 The project's VM by default, and update notices. Decided: commands
  that connect or read default to the project's VM; ones that destroy
  don't.** Inside a checkout, `ssh`, `code`, `get`, `open`, `ports`,
  `network ls` and `snapshot save`/`ls` use its VM when none is named,
  while `rm`, `rebuild` and `snapshot restore` still need a name, so a
  script run in a checkout never deletes or rolls back a VM it didn't
  name. Once a day, a command in a terminal starts `update-check` in the
  background, which asks GitHub for the latest release; the next command
  says when it's newer. It never blocks a command, never prints in
  scripts, `--json`, `-q` or MCP, and `UPDATE_CHECK=no` turns it off. The
  reply is data: a tag that isn't a version is refused before it's stored
  or shown. `project_init` gives Claude `init --project --print` for a git
  checkout only, so it can't read around the Mac.
- **D29 Working in the project's clone. Decided: in a checkout, `ssh` and
  Claude's `project_run` work in the VM's clone.** The clone is where the
  project's code is in the VM, so `limabean ssh -- make test` runs there,
  and a plain `limabean ssh` starts the shell there; a `--` before anything
  means "no VM name, this is the command". `project_run` finds the clone
  as `up` does (the origin read from `.git/config` as a file, the clone
  located with ghq in the VM) and runs the command there; like
  `run_in_vm`, it runs anything in the VM, so it carries the destructive
  hint. The Mac's checkout and the VM's clone stay separate copies, and
  the docs say so where it matters.
- **D30 Workflows as MCP prompts. Decided: a few whole tasks, written out
  as prompts.** `test_in_sandbox`, `triage_repo`, `try_a_change`,
  `run_untrusted` and `tidy_vms` show up as slash commands in Claude Code
  and in Claude Desktop's menu. A prompt is only text, so it adds no
  power: the tools' guards decide what happens. Each prompt also ends with
  the same rules (plans first, never widen a network, deleting and
  restoring left to the user), and a test checks that every tool a prompt
  names exists, so the prompts can't drift from the tools.
- **D31 What a VM knows about itself. Decided: a file and a script, no
  service.** limabean writes `~/.config/limabean/vm.json` in the VM (its
  name, environment, packs, tools and, for a project, the repository,
  clone, ports, var names, setup and start) when it creates or rebuilds a
  VM and on every project `up`, and `limabean-info` prints it. A service
  answering queries was the alternative; a file needs no daemon, no port
  and nothing to secure, and an agent reads it with `cat` or `jq`. It's in
  the user's home rather than `/etc` because a locked VM has no sudo for
  later writes. The network policy and the secrets' names aren't stored:
  `limabean-info` reads them live, the policy from the read-only mount
  (so a process in the VM can't make the file lie about its own limits)
  and the names from `secrets.env`. No secret value or reference goes in
  the file. It's only a description, so failing to write it warns and
  carries on.
- **D32 Getting work out of a VM. Decided: the VM's clone as a git remote
  of the Mac's checkout.** The VM's clone and the Mac's checkout are
  separate copies, and pushing through GitHub just to review a change is
  slow and needs the VM to reach GitHub. `limabean remote` adds
  `lima-<vm>:<clone>` as a remote, so `git fetch` copies the VM's branches
  over SSH, as `limabean ssh` reaches it, whatever the VM's network allows.
  It runs git on the Mac, so it's a command only the user runs, never an
  MCP tool. The clone's path comes from the VM and is data: it must be a
  plain absolute path before it goes into the Mac's git config, the
  remote's name can't be a flag, and the remote fetches no tags, so a VM
  can add branches under `limabean/` and nothing else. Fetching copies
  objects and runs nothing from the VM; checking a branch out is the
  user's choice, as with any remote. A remote of the same name that isn't
  a VM's is never changed or removed.
- **D33 What a VM knows, from the Mac. Decided: `limabean info` and the
  MCP `vm_info` tool print `limabean-info` (D31).** Claude on the Mac
  reads the same description an agent in the VM does, instead of working
  out the clone's path and tools by running commands. The VM's output is
  data: `--json` passes it on only if it's JSON. A VM made before
  limabean-info says it needs a rebuild.
- **D34 Claude Code in a VM. Decided: a status line and a note, through
  Claude Code's own settings, not a plugin.** A Claude Code mod (#37)
  showed the same kind of thing and was taken out (#40): its settings
  depended on Claude Code's version and on running in a terminal.
  `statusLine` is a plain setting that runs a script, so
  `limabean-statusline` works across versions; it reads the policy from
  the read-only mount, `vm.json`, git and `ss`, and sends nothing. A
  managed block in `~/.claude/CLAUDE.md` tells Claude how a limabean VM
  works (limabean-info, blocked hosts, ports, the clone, secrets).
  `features/claude.sh` sets both at each boot, only filling in what the
  user hasn't set, and turns off nonessential traffic a locked network
  would block. Permissions are left as Claude Code's defaults: skipping
  them is the user's call.
- **D35 Download progress. Decided: watch Lima's cache, not its output.**
  With `--tty=false` limactl prints "Downloading the image" and nothing
  more, and an interrupted download starts over, so a quiet spinner for
  15 minutes on a slow link got killed and retried. While a Lima step
  runs, limabean finds the newest `data.tmp.*` in Lima's download cache
  and reports its size, speed and time left. The total comes from one
  HEAD request for the `url` file beside it (https only): the URL is the
  templates' own image, and nothing from it runs. Parsing limactl's
  output was the alternative: it has no progress to parse.
  For the same reason limabean passes `limactl start --timeout=30m`:
  Lima's default of 10 minutes ran out during first boots with several
  packs on a slow link, while the VM was still installing. On a failure
  only the last lines of limactl's log are shown, without the lines it
  always prints.
- **D36 A company proxy's root. Decided: copy the Mac's admin-trusted
  roots into VMs, with `CA_CERTS=no` to opt out.** Proxies such as Zscaler
  re-sign HTTPS for some hosts with their own root; the Mac trusts it, a VM
  didn't, and mise spent an hour retrying `mise-versions.jdx.dev` before
  failing. When a VM or image is created, limabean reads the admin trust
  settings (`security trust-settings-export -d`) and passes the System
  keychain's certificates trusted there without conditions to Lima's
  `caCerts`, which installs them before provisioning. Roots trusted only
  for some uses, distrusted, expired or not CAs are left out, so a VM
  trusts nothing the Mac doesn't. A setting listing certificate files was
  the alternative: it works, but only for people who know they're behind
  a proxy, and they find out from a failed boot.
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
2. ✅ Tool packs (`--with`/`--without`, `PACKS`) instead of per-environment
   mise TOML files.
3. ✅ The CLI: `init`, `doctor`, `new`, `ls`, `ssh`, `get`, `stop`,
   `rebuild` and `rm`, with the unsaved-work guard.
4. Dotfiles (`features/dotfiles.sh`). ✅ Code-location option C (persistent
   disk).
5. ✅ `limabean bump` (rewrite pinned versions in `packs/*.toml`, monthly in
   `bump.yml`), ✅ CI (`limactl
   tmpl validate` + shellcheck), and ✅ a persistent `~/code` disk so
   rebuilds keep repositories.
6. ✅ Agent workflows: project files (`.limabean.yaml`, `up`), throwaway VMs
   (`--ttl`, `run --rm`), copy-on-write clones (`clone`, over `limactl
   clone`), learning allowlists from DNS lookups (`network learn`/`log`),
   `cp`, and a `claude` pack.
7. ✅ Images: new VMs are `limactl clone`s of a provisioned, cleaned and
   stopped VM, one per environment, pack set and templates hash, built in
   the background and replaced weekly (`limabean images`).
