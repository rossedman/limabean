# Packs: languages and tools

What a VM has installed is split into **packs**: Go, Node, Kubernetes, and
so on. Every VM has `core` (the shell tools limabean relies on), Docker and
Python; everything else is a pack you choose for that VM:

```bash
limabean packs                          # what there is
limabean up api                        # in a terminal, new asks which packs
limabean up api --with go,db           # or name them
limabean rebuild dev --with ruby        # add a pack to an existing VM
```

```yaml
# .limabean.yaml: a project's VM gets exactly these, besides core
packs: [go, db]
```

A VM keeps its packs through `limabean rebuild`, and `limabean ls --json`
lists them. Claude can choose them too: [`create_vm`](mcp.md#tools) takes
`with`. The [recipes](recipes.md) show which packs suit which kind of
project.

## The packs

| Pack | Installs |
|---|---|
| `core` (every VM) | ripgrep, fd, fzf, bat, delta, lazygit, gh, ghq, jq, yq, direnv, just, hyperfine, glow, gum, shellcheck, shfmt, svu |
| `go` | go, goreleaser, golangci-lint, gofumpt, goimports, gopls (`go vet` comes with go) |
| `python` | uv, ruff, yamllint, pyright (Python itself is in every VM, from apt) |
| `node` | node (LTS), pnpm, bun, typescript, typescript-language-server |
| `kubernetes` | kubectl, helm, kind, k9s |
| `infra` | terraform, tflint, aws-cli |
| `containers` | act, dive, skopeo (Docker itself is in every VM) |
| `db` | psql, sqlite3, redis-cli |
| `api` | buf, grpcurl |
| `supply-chain` | cosign, syft, grype |
| `rust` | rust (stable, through rustup, with rust-analyzer), cargo-binstall |
| `java` | Temurin JDK (LTS), gradle, maven |
| `ruby` | ruby (precompiled), and the libraries native gems build against |
| `elixir` | Erlang/OTP (precompiled), elixir |
| `dotnet` | the .NET SDK (C#, F#) |
| `security` | trivy, semgrep, gitleaks |
| `gcp` | gcloud (with gsutil and bq) |
| `azure` | az |
| `claude` | Claude Code, to run agents inside the VM |

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
limabean up agent --with claude --allow github --allow claude --locked
limabean get agent me/app
limabean ssh agent 'cd ~/code/github.com/me/app && claude -p "fix the failing test"'
```

With [`limabean run --rm`](lifecycle.md#throwaway-vms), the VM is gone when
the task is done. [Agents in a VM](agents.md) covers running Claude there
interactively in tmux, in the background and side by side.

### What Claude Code gets in a VM

Every VM sets up Claude Code for working there, at each boot:

- **A status line** at the bottom of Claude Code in the terminal, showing
  the VM, what its network allows, the branch (with `*` for changes), the
  ports reaching your Mac and the model:

    ```
    app · only github.com +2, locked · agent/fix* · :3000 · Opus 5.5
    ```

    The network part is read live from the policy the VM's firewall uses,
    so it's always what the VM can really reach.
- **A note in `~/.claude/CLAUDE.md`** telling Claude it's in a limabean
  VM: to run `limabean-info` before guessing what's installed; that a
  host it can't reach is probably blocked, and to name it rather than
  retry; that ports reach the Mac's `localhost`; to commit on a branch in
  the clone; and never to print a secret's value.
- **No nonessential traffic**: `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`,
  so Claude Code doesn't try update checks and error reports that a locked
  network would only block.

Your own choices win. A `statusLine` or `env` value you set in the VM's
`~/.claude/settings.json` is kept, and `CLAUDE.md` is yours outside
limabean's block, which is rewritten at each boot.

## Packs for every new VM

The `PACKS` setting in `~/.config/limabean/config` (or `LIMABEAN_PACKS`)
adds packs to every VM made with `limabean up`, `limabean run` or MCP's
`create_vm`. It's empty unless you set it; `up`'s form starts with these
checked, and `--without` leaves one out:

```bash
PACKS="go,node"
```

Project files ignore it: a `.limabean.yaml` lists every pack its VM gets,
so the VM is the same for everyone. Existing VMs keep their packs;
`limabean rebuild dev --with rust` adds one.

Earlier versions gave VMs Go, Python, Node, Kubernetes, Terraform, container and
database tools unless told otherwise, and `limabean init` wrote that list
into `PACKS`. If your config still has it, every new VM keeps getting
them; set `PACKS=""` for the smaller default.

## Language servers

The language packs carry their language server: gopls in `go`, pyright in
`python`, typescript-language-server in `node`, and rust-analyzer in
`rust`. Editors use them over [`limabean code`](inside-a-vm.md), and so does
Claude Code: Anthropic's LSP plugins give Claude go-to-definition, find
references and type errors as it edits. Turn them on in a project file:

```yaml
# .limabean.yaml
packs: [go, node]
claude:
  marketplaces:
    - marketplace: anthropics/claude-plugins-official
      plugins: [gopls-lsp@claude-plugins-official, typescript-lsp@claude-plugins-official]
```

The plugins for the others are `pyright-lsp` and `rust-analyzer-lsp`.

## Cloud CLIs

`infra` brings the AWS CLI, which uses your Mac's `~/.aws`, mounted
read-only. `gcp` and `azure` log in inside the VM, with a device code you
open on your Mac: `gcloud auth login --no-launch-browser` and
`az login --use-device-code`. Their credentials stay in that VM.

## Your own packs

A pack is a [mise](https://mise.jdx.dev) TOML file, and optionally a list of
apt packages. Put your own in `~/.config/limabean/templates/packs/`:

```toml
# ~/.config/limabean/templates/packs/zig.toml
# Zig and its language server.
[tools]
zig = "0.16.0"
zls = "0.16.0"   # zls follows zig's minor version
```

```text
# ~/.config/limabean/templates/packs/zig.apt  (optional)
libc6-dev
```

The first line is the description `limabean packs` shows. Then
`limabean up chat --with zig`. A tool with options is an inline table
whose first key is its version, as mise writes it:
`rust = { version = "1.99.1", components = "rust-analyzer" }`. A pack of yours with a built-in pack's
name replaces it, so `go.toml` there changes what the `go` pack installs.
`limabean bump ~/.config/limabean/templates/packs/*.toml` keeps your pins
current.

A pack lives on your Mac. For tools one project needs, list them in its
`.limabean.yaml` instead, under [`tools` and `apt`](projects.md#tools-and-apt),
so everyone who checks it out gets them.

## Sandboxes

A VM with a [network allowlist](network.md) needs its packs' registries to
install packages at work time. Presets name them: `go`, `npm`, `python`,
`rust`, `java`, `ruby`, `elixir` (Hex), `dotnet` (NuGet), and `gcp` and
`azure` for their CLIs. Packs themselves install on the first boot, before
the allowlist applies.

## How it works

`layer-base.yaml` writes `core` into the VM as
`/etc/limabean/mise/00-core.toml`. `limabean up` adds each other pack with a
`--set` provision, as `/etc/limabean/mise/50-<pack>.toml`, plus
`/etc/limabean/apt/<pack>.list` for its apt packages. On first boot,
`features/packs-apt.sh` installs the apt packages and `features/mise.sh`
links every TOML into `~/.config/mise/conf.d` and runs `mise install`.
limabean remembers each VM's packs in `~/.local/state/limabean/<vm>.packs`
so a rebuild gets them again.
