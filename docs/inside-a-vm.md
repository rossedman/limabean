# Inside a VM

Each VM runs Ubuntu 26.04 LTS, with systemd 259 and its Varlink APIs, fully
updated on first boot. It's small by default: 2 CPUs, 4 GiB of memory and
a 10 GiB system disk, with `~/code` on a 50 GiB disk of its own. Docker
images live on the system disk, so give a VM that builds big ones more with
`limabean up <name> --disk 40`, or `DISK` in the config for every VM. It has no access to your Mac's files apart from a
few read-only mounts: your git config, `~/.aws` if you have it, and the VM's
own [network policy](network.md). Keep code in the VM under `~/code`, which is
on its own disk and [survives rebuilds](lifecycle.md), and push it to a
remote.

## Tools

- **Docker Engine** from Docker's apt repository, with the buildx and compose
  plugins. BuildKit is the default builder, so custom Dockerfile syntaxes
  (`# syntax=docker/dockerfile:1`) work. Your user owns the socket, so `docker`
  needs no sudo, and kind and testcontainers work as they do on a Mac.
- **mise**, with the tools of the VM's [packs](packs.md), every version
  pinned. Every VM has the core shell tools (ripgrep, fd, fzf, bat, delta,
  lazygit, gh, ghq, just, shellcheck, shfmt, …); Go, Node, Python's uv,
  Kubernetes, Terraform, Rust, Java, Ruby and more are packs you pick.
  Use `mise use node@22` or `mise use terraform@1.9` for other versions.
- **Language servers** for Go, Python, TypeScript and Rust, in their
  packs, for your editor and for Claude's LSP plugins
  ([packs](packs.md#language-servers)).
- **Formatters and linters** for Go, Python, shell and YAML: gofumpt,
  goimports, golangci-lint and `go vet`; ruff and yamllint; shellcheck and
  shfmt. shellcheck and shfmt are in every VM; the rest come with the `go`
  and `python` packs.
- **apt**: git, git-extras, git-quick-stats, curl, wget, vim, tmux, stow,
  httpie, jq, tree, htop, watch, speedtest-cli, build-essential, python3, pip,
  and venv.
- **Rosetta** on Apple Silicon, so x86_64 Linux programs run as they are:
  a downloaded `amd64` binary, or `docker run --platform linux/amd64 …` for
  an image with no arm64 build. It's slower than native, so prefer arm64
  images when they exist. Intel Macs run x86_64 natively and skip it.
  If creating a VM stops at `Installing rosetta...`, run
  `softwareupdate --install-rosetta` on the Mac.

## Ports

Lima forwards every TCP port a program opens on localhost or on all
addresses in the VM to the same port on your Mac's localhost. A dev server
on port 3000, or a container started with `-p 8080:80`, is at
`http://localhost:3000` or `http://localhost:8080` on the Mac.
`limabean ports dev` lists them and shows which reach the Mac (a port a Mac
program already uses doesn't), and `limabean open dev 3000` opens one in
your browser.

## Updates

The first boot runs a full `apt-get full-upgrade`, systemd included, and
switches the running systemd to the new version. If an update needs a restart,
for example a new kernel, `limabean up` restarts the VM before handing it
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
  [`completions.sh`](https://github.com/rossedman/limabean/blob/main/templates/features/completions.sh)

Put personal shell additions in `~/.zshrc.local`. `~/.zshrc` itself is
rewritten on every boot.

## Paths

[`env.sh`](https://github.com/rossedman/limabean/blob/main/templates/files/env.sh) is loaded by every shell, interactive
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

## Copying files

`limabean cp` copies files between your Mac and a VM, or between VMs. A VM
path is `<vm>:<path>`, relative to the VM user's home directory:

```bash
limabean cp ./notes.md dev:
limabean cp -r dev:code/github.com/me/app/dist ./dist
limabean cp dev:report.html agent:
```

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

The global [`.editorconfig`](https://github.com/rossedman/limabean/blob/main/templates/files/editorconfig) applies unless a
project has its own.
