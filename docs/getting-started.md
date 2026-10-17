# Getting started

## Install

limabean needs macOS, [Lima](https://lima-vm.io) 2.0 or newer, and `jq` (for
Claude Desktop integration).

```bash
brew install lima jq
git clone https://github.com/rossedman/limabean ~/code/github.com/rossedman/limabean
ln -s ~/code/github.com/rossedman/limabean/bin/limabean ~/.local/bin/limabean   # any directory on your PATH
```

`limabean` is a single bash script. It finds the templates next to itself, so
keep the symlink pointing into the clone.

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
limabean new dev                       # about a minute; longer the first time while Lima downloads Ubuntu
limabean ssh dev ssh -T git@github.com # should greet you by your GitHub username
limabean get dev owner/repo            # clone into ~/code/github.com/owner/repo
```

Next: [the CLI](cli.md), [what's inside a VM](inside-a-vm.md) and
[Claude Desktop](claude-desktop.md).
