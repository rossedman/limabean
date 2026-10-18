# Getting started

## Install

limabean needs macOS and [Lima](https://lima-vm.io) 2.0 or newer. It's a
single binary with the VM templates built in.

**With Homebrew** (installs Lima too):

```bash
brew install rossedman/tap/limabean
brew upgrade limabean   # later, for a new release
```

The other ways need Lima first: `brew install lima`.

**From a release.** The download works on Apple Silicon and Intel:

```bash
# download limabean_<version>_darwin_universal.tar.gz from
# https://github.com/rossedman/limabean/releases, then:
tar -xzf limabean_*_darwin_universal.tar.gz limabean
xattr -d com.apple.quarantine limabean 2>/dev/null   # downloaded binaries are quarantined by macOS
mv limabean ~/.local/bin/                            # any directory on your PATH
```

**With Go:**

```bash
go install github.com/rossedman/limabean/cmd/limabean@latest
```

For tab completion in zsh: `limabean completion zsh > "${fpath[1]}/_limabean"`.
`limabean completion --help` covers other shells.

## Set up

```bash
limabean init
```

`init` asks for the host directory that holds your git config and where
repositories should go inside VMs. It saves both to `~/.config/limabean/config`,
offers to add the SSH `Include` described below, and then runs
`limabean doctor`. Run `limabean doctor` any time to recheck everything.

## Requirements

`doctor` checks each of these and prints the exact fix.

### SSH config includes Lima's host entries

Lima writes an SSH config for each VM to `~/.lima/<name>/ssh.config` and
rewrites it on every start, because the forwarded port changes. Include those
files so every VM is reachable as `lima-<name>`:

```
Include ~/.lima/*/ssh.config
```

The line must sit **at the top** of `~/.ssh/config`, before any `Host` or
`Match` block. An `Include` inside a block only applies to that block.

### Your SSH key is loaded in the agent

VMs never receive a copy of your private keys. Lima forwards your Mac's
ssh-agent instead, so git inside a VM can push as you while the key is loaded:

```bash
ssh-add --apple-use-keychain ~/.ssh/id_ed25519
```

To load it automatically after a reboot, add this below the `Include` line:

```
Host *
  AddKeysToAgent yes
  UseKeychain yes
```

With another agent, such as 1Password or Secretive, make sure `SSH_AUTH_SOCK`
points at it in the shell where you run `limabean new`.

### Your git config is in a directory

Lima mounts directories, not single files. Each VM mounts one host directory
read-only at `/mnt/host/git`, and the VM's `~/.gitconfig` includes the config
found there. Edits on the Mac show up in every VM immediately.

| Your setup | What to do |
|---|---|
| `~/.config/git/config` (git's XDG location) | Nothing, it's the default. |
| `~/.gitconfig` as a plain file | `mkdir -p ~/.config/git && mv ~/.gitconfig ~/.config/git/config`. Git still finds it. |
| `~/.gitconfig` symlinked from a dotfiles repo | Nothing. `init` follows the symlink and suggests that directory. |

In that directory the VM looks for, first match wins:

- config: `config`, `.gitconfig`
- global ignore: `ignore`, `.gitignore`
- commit template: `.gitcommit.txt`, `commit-template.txt`

Settings that only make sense on the Mac are overridden in the VM:
`core.editor` becomes `vim`.

## Your first VM

```bash
limabean new dev                       # a few minutes the first time; later VMs are copies of an image
limabean ssh dev ssh -T git@github.com # should greet you by your GitHub username
limabean get dev owner/repo            # clone into ~/code/github.com/owner/repo
```

The first VM provisions everything from scratch, and builds an
[image](lifecycle.md#fast-vms-images) of itself in the background. Every
later VM like it is a copy of that image, so it only has to boot.

For `gh` and the GitHub API inside VMs, add a scoped token once with
`limabean github login`; see [GitHub access](github.md).

## Connect Claude

Every VM already shows up in Claude Desktop as an SSH connection: pick
**SSH → `dev (limabean)`** in the prompt box's environment menu
([details](claude-desktop.md)).

To let Claude manage VMs itself, add limabean's MCP server to Claude Code:

```bash
claude mcp add --scope user limabean -- limabean mcp
```

Then ask Claude to *"make a VM, clone owner/repo and run the tests"*. Setup
for Claude Desktop, example prompts and what Claude can and can't do are in
[Claude and MCP](mcp.md).

## A VM for each project

Put a `.limabean.yaml` in a repository to describe the VM it needs (its
packs, size, network, secrets and setup), and `limabean up` in the checkout
creates it, clones the repository and sets it up:

```yaml
# .limabean.yaml
packs: [rust]
ports: [3000]
setup: |
  make deps
```

To start from a [recipe](recipes.md) instead of a blank file:

```bash
limabean recipes                 # go-service, node-web, python-api, rails, claude-agent, …
limabean recipes go-service      # writes .limabean.yaml here; edit it, then limabean up
```

See [Projects](projects.md).

## Next

- [Claude and MCP](mcp.md): let Claude create and drive VMs
- [Projects](projects.md): `.limabean.yaml` and `limabean up`
- [Recipes](recipes.md): starter project files for common kinds of VMs
- [Packs](packs.md): Go, Node, Rust, Java, Ruby, Claude Code and more
- [Sandboxes](network.md): limit what a VM can reach, and learn what it needs
- [Rebuilds, clones and snapshots](lifecycle.md): fast images, throwaway VMs, autostop
- [Inside a VM](inside-a-vm.md): tools, Docker, shell, ports, copying files
- [The CLI](cli.md): every command and setting
