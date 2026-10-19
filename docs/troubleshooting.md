# Troubleshooting

Start with `limabean doctor`. It checks every prerequisite and prints the fix.

| Symptom | Cause and fix |
|---|---|
| `ssh: Could not resolve hostname lima-<name>` | The `Include ~/.lima/*/ssh.config` line is missing, or it sits inside a `Host` block. It has to be at the top of `~/.ssh/config`. |
| `git@github.com: Permission denied (publickey)` in a VM | No key is loaded in your Mac's agent. Check with `ssh-add -l`, and see [getting started](getting-started.md#your-ssh-key-is-loaded-in-the-agent). |
| `limabean: no git config found` in the boot log | `GIT_CONFIG_DIR` points at a directory without a `config` or `.gitconfig`. |
| A download or `git clone` fails in a VM with an allowlist (`Could not resolve host`) | The domain isn't allowed: `limabean network log <name> --blocked` shows what was blocked and the command that allows it. Installers often fetch from a second domain, such as a CDN. |
| `http://localhost:<port>` on the Mac doesn't reach a server in the VM | Run `limabean ports <name>`. If the port isn't listed, the server listens on another address: start it on `127.0.0.1` or `0.0.0.0` (often `--host 0.0.0.0`). If it says not forwarded, a program on the Mac already has that port. |
| `gh` in a VM says you're not logged in | No token is saved, or the VM missed it while stopped: `limabean github status`, then `limabean github login` or `sync`. |
| `gh` or `limabean doctor` reports a bad credential | The token expired or was revoked. Create a new one and run `limabean github login`. |
| `git push` fails in a Claude Desktop session (`Permission denied (publickey)`) | Claude sessions have no forwarded ssh-agent. Save a token with `limabean github login`; git then pushes over https. VMs created before this need `limabean rebuild`. |
| `aws` in a VM can't refresh an SSO session | `~/.aws` is read-only in VMs. Run `aws sso login` on your Mac. |
| `limabean` prints plain text and doesn't prompt | Output isn't a terminal (a pipe or script), or `--plain` / `LIMABEAN_PLAIN=1` is set. |
| A secret is missing in a VM | `limabean secrets ls --vm <name>` shows what it should get, and `limabean doctor` checks every reference. A stopped VM gets changes at its next `limabean secrets sync`. |
| `op couldn't read it` during sync | 1Password is locked or the op CLI isn't signed in. Unlock it and run `limabean secrets sync` again. |
| macOS says limabean "cannot be opened" after downloading | The binary is quarantined (Homebrew clears this itself): `xattr -d com.apple.quarantine ~/.local/bin/limabean`. |
| The editor says `Unable to resolve resource vscode-remote://ssh-remote+lima-…` | It has no Remote - SSH extension; VS Code Insiders and VSCodium don't share VS Code's extensions. `limabean code` offers to install it, or run `code-insiders --install-extension ms-vscode-remote.remote-ssh` (`codium --install-extension jeanp413.open-remote-ssh`). |
| A VM doesn't appear in Claude Desktop | Only running VMs are listed: `limabean up <name>`. Otherwise restart Claude Desktop, or run `limabean claude sync`. |
| A project's `start` doesn't seem to run | `limabean logs --start` shows its output. `up` doesn't run changed start commands until you approve them, and leaves one that's still running alone. |
| `limabean up` hangs on "mise tools are installed" | Read the provisioning log: `limabean logs <name>` (`-f` to follow it). |
| Downloads in a VM fail with `certificate verify failed`, `UnknownIssuer` or `self-signed certificate in chain` | A company proxy (Zscaler and the like) inspects HTTPS and signs with its own root. limabean gives new VMs the roots an admin told the Mac to trust, so check the Mac has it: `security trust-settings-export -d /tmp/t.plist`. VMs created before limabean did this need `limabean rebuild <name>`. A root you trust only in your login keychain isn't copied. |
| `limabean up` sits on "Downloading …img" | The first VM downloads Ubuntu's cloud image, about 900 MiB; the line under the spinner shows how long is left. Let it finish: Lima starts an interrupted download over. |
| `Lima stopped waiting after 30m0s, but the VM is still provisioning` | A first boot with several packs took longer than limabean waits, often on a slow connection. The VM carries on: follow it with `limabean logs <name> -f`. |
| `limabean up` finishes but the VM is stopped | limabean starts it again, and says so if it still won't run. Lima's log for it: `limabean logs <name> --lima`. |
| A new VM wasn't fast | It was the first of its kind (environment and packs), or the templates changed since its image was built: an image is being built in the background (`~/Library/Logs/limabean-images.log`). `limabean images ls` shows what there is. |
| A VM runs out of disk (`no space left on device`) | Docker images live on the 10 GiB system disk; `~/code` has its own. `docker system prune` frees space; for more, set `DISK="30"` (or `up --disk 30`) and `limabean rebuild <name>`. |
| `limabean up` says to run again with `--yes` | It needs your OK to act on the file, and there's no terminal to ask in. Read the plan it printed, then pass `--yes`. |
| `setup` fails in `limabean up` | The VM is kept: `limabean ssh <name>` to look. With `network.allow`, setup can only reach those domains (`limabean network log <name>` shows what it tried); with `locked`, it has no sudo or Docker. Fix the file, then run it again: `limabean up --setup`. |
| `rm` or `rebuild` refuses to run | A repository under `~/code` has unpushed work, listed in the output. Push it, or pass `--force`. (`rebuild` only checks VMs without a code disk, whose `~/code` it would erase.) |
