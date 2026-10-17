# Troubleshooting

Start with `limabean doctor`. It checks every prerequisite and prints the fix.

| Symptom | Cause and fix |
|---|---|
| `ssh: Could not resolve hostname lima-<name>` | The `Include ~/.lima/*/ssh.config` line is missing, or it sits inside a `Host` block. It has to be at the top of `~/.ssh/config`. |
| `git@github.com: Permission denied (publickey)` in a VM | No key is loaded in your Mac's agent. Check with `ssh-add -l`, and see [getting started](getting-started.md#your-ssh-key-is-loaded-in-the-agent). |
| `limabean: no git config found` in the boot log | `GIT_CONFIG_DIR` points at a directory without a `config` or `.gitconfig`. |
| `gh` in a VM says you're not logged in | No token is saved, or the VM missed it while stopped: `limabean github status`, then `limabean github login` or `sync`. |
| `gh` or `limabean doctor` reports a bad credential | The token expired or was revoked. Create a new one and run `limabean github login`. |
| `git push` fails in a Claude Desktop session (`Permission denied (publickey)`) | Claude sessions have no forwarded ssh-agent. Save a token with `limabean github login`; git then pushes over https. VMs created before this need `limabean rebuild`. |
| A new VM doesn't appear in Claude Desktop | Restart Claude Desktop, or run `limabean claude sync`. |
| `limabean new` hangs on "mise tools are installed" | Read the provisioning log: `limactl shell <name> sudo tail -50 /var/log/cloud-init-output.log`. |
| `rm` or `rebuild` refuses to run | A repository under `~/code` has unpushed work, listed in the output. Push it, or pass `--force`. |
