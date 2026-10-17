# GitHub access

The `gh` CLI, the GitHub API, private clones and authenticated downloads (mise,
Go) need a GitHub token, and so does pushing from Claude Desktop sessions.
limabean keeps one token in your macOS Keychain and installs it in every VM.

```bash
limabean github login     # paste a token; installs it in running VMs
limabean new dev          # new VMs get it automatically
```

## Use a fine-grained token

Anything running in a VM, including Claude, can read the token. Give it as
little access as possible:

1. Create a [fine-grained token](https://github.com/settings/personal-access-tokens/new).
2. Limit it to the repositories you'll work on in VMs.
3. Give it an expiry date.
4. Grant only what you need, for example **Contents** and **Pull requests**:
   read and write.

`limabean github login` accepts other tokens, such as `gh auth token`, but warns
you: they usually carry broad scopes like `repo` and `workflow` across every
repository you can reach.

## Commands

| Command | What it does |
|---|---|
| `limabean github login [--with-token]` | Checks the token with GitHub, saves it to the Keychain and installs it in running VMs. Prompts for the token, or reads it from stdin with `--with-token` |
| `limabean github status` | Shows the token's account and expiry, and which VMs have it |
| `limabean github sync` | Installs the saved token in every running VM, after rotating it or starting a stopped VM |
| `limabean github logout` | Removes the token from the Keychain and from running VMs |

`limabean doctor` reports the token's account and expiry, and flags a token
that has expired or been revoked.

## Passing the token on stdin

`--with-token` reads the token from stdin instead of prompting, the same as
`gh auth login --with-token`. Use it in scripts or with a password manager:

```bash
op read "op://Private/GitHub limabean/token" | limabean github login --with-token
security find-generic-password -s my-github-token -w | limabean github login --with-token
gh auth token --hostname github.com | limabean github login --with-token
```

The last line reuses your host `gh` login, which is quick but usually broadly
scoped (see above), so limabean warns about it. With `--with-token` and nothing
piped in, limabean stops and says so instead of waiting. Surrounding whitespace
and a missing trailing newline are fine. Even without the flag, a token piped
into `limabean github login` is read from stdin.

## How git reaches GitHub

| | With a token | Without a token |
|---|---|---|
| Fetch | https, with the token | https (public repositories only) |
| Push | https, with the token | SSH, through your forwarded ssh-agent |
| Remotes like `git@github.com:…` | rewritten to https | SSH |
| Claude Desktop sessions | work | can't push |

Claude Desktop sessions don't get a forwarded ssh-agent, so SSH pushes fail
there. With a token installed, git in the VM rewrites GitHub SSH remotes to
https and uses the token, so pushing works the same everywhere. The rule lives
in `~/.config/limabean/github.gitconfig`. `limabean-github-git` rewrites it at
boot and whenever `limabean github` installs or removes the token. The token
needs **Contents: read and write** on a repository to push to it.

## How it works

- **On the Mac**, the token is a Keychain item named `limabean-github-token`.
  It's passed to `security` and `ssh` on stdin, so it never appears in a
  process list. It doesn't depend on your host `gh` login.
- **In the VM**, it's written to `~/.config/limabean/github-token` (mode 600).
  [`env.sh`](../templates/files/env.sh) exports it as `GITHUB_TOKEN` in every
  shell, interactive or not. gh uses `GITHUB_TOKEN` automatically, and git uses
  gh as its credential helper for `https://github.com`.
- **Never in a template.** Lima keeps templates in plain text under `~/.lima`,
  so the token is copied in after the VM boots. Provisioning on first boot
  therefore runs without it.

When the token expires, run `limabean github login` with a new one. It updates
the Keychain and every running VM.
