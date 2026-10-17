# Inside a VM

Each VM runs Ubuntu 26.04 LTS, with systemd 259 and its Varlink APIs, fully
updated on first boot. It has no access to
your Mac's files apart from the read-only git config mount. Keep code in the VM
under `~/code` and push it to a remote.

## Tools

- **Docker Engine** from Docker's apt repository, with the buildx and compose
  plugins. BuildKit is the default builder, so custom Dockerfile syntaxes
  (`# syntax=docker/dockerfile:1`) work. Your user owns the socket, so `docker`
  needs no sudo, and kind and testcontainers work as they do on a Mac.
- **mise**, with every version pinned in
  [`templates/mise/base.toml`](../templates/mise/base.toml): go, goreleaser,
  golangci-lint, uv, kubectl, helm, kind, k9s, terraform, aws-cli, cosign,
  syft, grype, ripgrep, fd, fzf, bat, gh, ghq, yq, direnv, glow, gum,
  shellcheck and svu. Use `mise use python@3.12` or `mise use terraform@1.9`
  for other versions.
- **apt**: git, git-extras, git-quick-stats, curl, wget, vim, tmux, stow,
  httpie, jq, tree, htop, watch, speedtest-cli, build-essential, python3, pip,
  venv and skopeo.
- **Rosetta** on Apple Silicon, so x86_64 Linux programs run as they are:
  a downloaded `amd64` binary, or `docker run --platform linux/amd64 …` for
  an image with no arm64 build. It's slower than native, so prefer arm64
  images when they exist. Intel Macs run x86_64 natively and skip it.
  If creating a VM stops at `Installing rosetta...`, run
  `softwareupdate --install-rosetta` on the Mac.

## Updates

The first boot runs a full `apt-get full-upgrade`, systemd included, and
switches the running systemd to the new version. If an update needs a restart,
for example a new kernel, `limabean new` restarts the VM before handing it
over. Later boots skip the upgrade: `limabean rebuild` is how a VM gets newer
packages.

## Shell

zsh is the login shell, with a single-line prompt built from zsh's own
`vcs_info`:

```
dev:~/c/g/r/limabean (main*) ❯
```

The prompt shows the VM name, then the path with parent directories shortened
to one letter, then the git branch. After the branch, `*` means unstaged
changes, `+` staged and `?` untracked. `❯` turns red after a failed command.

Also included:

- autosuggestions, syntax highlighting and shared history
- fzf on `ctrl-r` (history), `ctrl-t` (files) and `alt-c` (directories)
- `repo [query]` to jump to any ghq repository
- `cat` is bat without the header or pager. Run the real one with
  `command cat`. bat uses your terminal's colors.
- tab completion for nearly every tool, generated at boot by
  [`completions.sh`](../templates/features/completions.sh)

Put personal shell additions in `~/.zshrc.local`. `~/.zshrc` itself is
rewritten on every boot.

## Paths

[`env.sh`](../templates/files/env.sh) is loaded by every shell, interactive
or not, so `ssh lima-dev <cmd>` sees the same tools as a login shell.

| Variable | Value |
|---|---|
| `GOPATH` / `GOBIN` | `~/go` / `~/go/bin` |
| `PYTHONUSERBASE` | `~/.local`, where `pip install --user`, `uv tool install` and pipx put scripts (`~/.local/bin`) |
| `PATH` | `$GOBIN`, `~/.local/bin`, mise shims, then the system |
| `GITHUB_TOKEN` | the token from [`limabean github login`](github.md), if one is saved |

## Repositories

[ghq](https://github.com/x-motemen/ghq) clones into `~/code/<host>/<owner>/<repo>`.
`~/code` is a separate disk (a Lima disk named `code-<hash>`, mounted over
it at boot), so it survives `limabean rebuild`; `df -h ~/code` shows it.
How git reaches GitHub depends on whether a [GitHub token](github.md) is
installed. With one, everything goes over https with the token, so it works
in Claude Desktop sessions too. Without one, repositories are fetched over
https and pushed over SSH through your forwarded agent.

## AWS

If `~/.aws` exists on your Mac, it's mounted read-only at `~/.aws` in new
VMs, so the `aws` CLI uses your profiles. It's read-only because a writable
mount would let the VM change `~/.aws/config`, and its `credential_process`
setting runs commands on your Mac. That means:

- run `aws sso login` on the Mac; VMs see the refreshed session
- anything in the VM, including Claude, can use these credentials, so prefer
  profiles with limited permissions
- set `AWS_DIR=""` in `~/.config/limabean/config` to stop mounting it, or point
  it at a different directory

The global [`.editorconfig`](../templates/files/editorconfig) applies unless a
project has its own.
