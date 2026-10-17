# Packs: languages and tools

What a VM has installed is split into **packs**: Go, Node, Kubernetes, and
so on. Every VM gets a sensible default set, and you add or remove packs per
VM:

```bash
limabean packs                                    # what there is, and the defaults
limabean new api --with rust,java                 # the defaults, plus Rust and Java
limabean new web --without kubernetes,infra       # the defaults, minus two
limabean rebuild dev --with ruby                  # add a pack to an existing VM
```

A VM keeps its packs through `limabean rebuild`, and `limabean ls --json`
lists them. Claude can choose them too: [`create_vm`](mcp.md#tools) takes
`with`.

## The packs

| Pack | Default | Installs |
|---|---|---|
| `core` | always | ripgrep, fd, fzf, bat, delta, lazygit, gh, ghq, yq, direnv, just, hyperfine, glow, gum, shellcheck, svu |
| `go` | yes | go, goreleaser, golangci-lint |
| `python` | yes | uv, ruff (Python itself is in every VM, from apt) |
| `node` | yes | node (LTS), pnpm, bun |
| `kubernetes` | yes | kubectl, helm, kind, k9s |
| `infra` | yes | terraform, tflint, aws-cli |
| `containers` | yes | act, dive, skopeo (Docker itself is in every VM) |
| `db` | yes | psql, sqlite3, redis-cli |
| `api` | | buf, grpcurl |
| `supply-chain` | | cosign, syft, grype |
| `rust` | | rust (stable, through rustup), cargo-binstall |
| `java` | | Temurin JDK (LTS), gradle, maven |
| `ruby` | | ruby (precompiled), and the libraries native gems build against |
| `claude` | | Claude Code, to run agents inside the VM |

`core` holds the tools limabean and the shell rely on, so it can't be
removed. Docker, Python 3 and the [apt basics](inside-a-vm.md#tools) are in
every VM whatever its packs.

Every version is pinned in
[`templates/packs/`](https://github.com/rossedman/limabean/tree/main/templates/packs).
For another version in one project, use mise: `mise use node@22` or
`mise use java@21`.

## Claude Code in a VM

The `claude` pack installs [Claude Code](https://code.claude.com) in the VM,
so an agent can work there instead of on your Mac. Give it a key with a
secret: `ANTHROPIC_API_KEY`, or `CLAUDE_CODE_OAUTH_TOKEN` from
`claude setup-token` on your Mac to use your subscription:

```bash
claude setup-token   # on the Mac; copy the token it prints
limabean secrets set CLAUDE_CODE_OAUTH_TOKEN --vm agent
limabean new agent --with claude --allow github --allow claude --locked
limabean get agent me/app
limabean ssh agent 'cd ~/code/github.com/me/app && claude -p "fix the failing test"'
```

With [`limabean run --rm`](lifecycle.md#throwaway-vms), the VM is gone when
the task is done.

## Changing the defaults

The defaults are the `PACKS` setting in `~/.config/limabean/config` (or
`LIMABEAN_PACKS`):

```bash
PACKS="go,python,node,kubernetes,infra,containers,db"
```

Add `rust` there, for example, and every new VM gets Rust. Existing VMs keep
their packs; `limabean rebuild dev --with rust` adds it to one.

## Your own packs

A pack is a [mise](https://mise.jdx.dev) TOML file, and optionally a list of
apt packages. Put your own in `~/.config/limabean/templates/packs/`:

```toml
# ~/.config/limabean/templates/packs/elixir.toml
# Elixir and Erlang.
[tools]
erlang = "28.1"
elixir = "1.19.0"
```

```text
# ~/.config/limabean/templates/packs/elixir.apt  (optional)
inotify-tools
```

The first line is the description `limabean packs` shows. Then
`limabean new chat --with elixir`. A pack of yours with a built-in pack's
name replaces it, so `go.toml` there changes what the `go` pack installs.
`limabean bump ~/.config/limabean/templates/packs/*.toml` keeps your pins
current.

## Sandboxes

A VM with a [network allowlist](network.md) needs its packs' registries to
install packages at work time. Presets name them: `go`, `npm`, `python`,
`rust`, `java` and `ruby`. Packs themselves install on the first boot, before
the allowlist applies.

## How it works

`layer-base.yaml` writes `core` into the VM as
`/etc/limabean/mise/00-core.toml`. `limabean new` adds each other pack with a
`--set` provision, as `/etc/limabean/mise/50-<pack>.toml`, plus
`/etc/limabean/apt/<pack>.list` for its apt packages. On first boot,
`features/packs-apt.sh` installs the apt packages and `features/mise.sh`
links every TOML into `~/.config/mise/conf.d` and runs `mise install`.
limabean remembers each VM's packs in `~/.local/state/limabean/<vm>.packs`
so a rebuild gets them again.
