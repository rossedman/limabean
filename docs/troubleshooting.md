# Troubleshooting

Start with `limabean doctor`. It checks every prerequisite and prints the fix.

| Symptom | Cause and fix |
|---|---|
| `ssh: Could not resolve hostname lima-<name>` | The `Include ~/.lima/*/ssh.config` line is missing, or it sits inside a `Host` block. It has to be at the top of `~/.ssh/config`. |
| `git@github.com: Permission denied (publickey)` in a VM | No key is loaded in your Mac's agent. Check with `ssh-add -l`, and see [getting started](getting-started.md#your-ssh-key-is-loaded-in-the-agent). |
| `limabean: no git config found` in the boot log | `GIT_CONFIG_DIR` points at a directory without a `config` or `.gitconfig`. |
| A download or `git clone` fails in a VM with an allowlist (`Could not resolve host`) | The domain isn't allowed: `limabean network ls <name>` shows what is, and `limabean network allow <name> <domain>` adds one. Installers often fetch from a second domain (a CDN); its name is in the error. |
| `http://localhost:<port>` on the Mac doesn't reach a server in the VM | Run `limabean ports <name>`. If the port isn't listed, the server listens on another address: start it on `127.0.0.1` or `0.0.0.0` (often `--host 0.0.0.0`). If it says not forwarded, a program on the Mac already has that port. |
| `gh` in a VM says you're not logged in | No token is saved, or the VM missed it while stopped: `limabean github status`, then `limabean github login` or `sync`. |
| `gh` or `limabean doctor` reports a bad credential | The token expired or was revoked. Create a new one and run `limabean github login`. |
| `git push` fails in a Claude Desktop session (`Permission denied (publickey)`) | Claude sessions have no forwarded ssh-agent. Save a token with `limabean github login`; git then pushes over https. VMs created before this need `limabean rebuild`. |
| `aws` in a VM can't refresh an SSO session | `~/.aws` is read-only in VMs. Run `aws sso login` on your Mac. |
| `limabean` prints plain text and doesn't prompt | Output isn't a terminal (a pipe or script), or `--plain` / `LIMABEAN_PLAIN=1` is set. |
| A secret is missing in a VM | `limabean secrets ls --vm <name>` shows what it should get, and `limabean doctor` checks every reference. A stopped VM gets changes at its next `limabean secrets sync`. |
| `op couldn't read it` during sync | 1Password is locked or the op CLI isn't signed in. Unlock it and run `limabean secrets sync` again. |
| macOS says limabean "cannot be opened" after downloading | The binary is quarantined: `xattr -d com.apple.quarantine ~/.local/bin/limabean`. |
| A VM doesn't appear in Claude Desktop | Only running VMs are listed: `limabean start <name>`. Otherwise restart Claude Desktop, or run `limabean claude sync`. |
| `limabean new` hangs on "mise tools are installed" | Read the provisioning log: `limactl shell <name> sudo tail -50 /var/log/cloud-init-output.log`. |
| `rm` or `rebuild` refuses to run | A repository under `~/code` has unpushed work, listed in the output. Push it, or pass `--force`. (`rebuild` only checks VMs without a code disk, whose `~/code` it would erase.) |
