# Agents in a VM: Claude Code, tmux and headless runs

The [walkthrough](walkthrough.md) keeps Claude on your Mac and lets it
drive a VM through MCP, asking you before each step. This page puts the
agent **inside** the VM instead. The VM is the sandbox: it can only reach
the hosts you allow, it has no sudo to change that, and it can't see your
Mac's files. That's what makes it reasonable to let Claude work without
asking permission for every command.

It covers five patterns, from hands-on to hands-off:

| Pattern | Use it for |
|---|---|
| [Interactive, in tmux](#1-interactive-claude-code-in-tmux) | Working alongside Claude, and leaving it running when you close the terminal |
| [One headless task](#2-one-headless-task) | A single job from your Mac's terminal or a script |
| [A long run in the background](#3-a-long-run-in-the-background) | Hours of work you check on now and then |
| [A throwaway VM](#4-a-throwaway-vm-for-one-task) | A task that should leave nothing behind |
| [Attempts side by side](#5-attempts-side-by-side) | Trying the same task more than once, or several tasks at once |

Each one ends the same way: the work is on a branch in the VM, and you
[bring it to your Mac](#getting-the-work-out).

## Set up a VM for agents

Once per Mac, give VMs a Claude login. `claude setup-token` uses your
subscription; an `ANTHROPIC_API_KEY` secret works too:

```bash
claude setup-token                               # on the Mac; copy the token it prints
limabean secrets set CLAUDE_CODE_OAUTH_TOKEN --vm app
```

`--vm app` gives the token to the VM named `app` only. Leave it out to
give it to every VM, which the [throwaway pattern](#4-a-throwaway-vm-for-one-task)
needs.

Then, in your repository's checkout on the Mac, start from the
[`claude-agent` recipe](recipes.md): Claude Code and the common languages,
a network locked to GitHub, the package registries and Anthropic, and no
sudo or Docker:

```bash
cd ~/src/app
limabean recipes claude-agent      # writes .limabean.yaml; edit packs and allow to fit
limabean up                        # creates the VM "app" and clones the repository into it
limabean snapshot save             # a clean starting point to roll back to
```

The VM is named after the checkout's directory, here `app`, and the
repository is cloned to `~/code/github.com/<owner>/app` in it. Inside the
checkout, the commands below find that VM and work in that clone without
being told.

## 1. Interactive Claude Code in tmux

[tmux](https://github.com/tmux/tmux) is in every VM. Running Claude in a
tmux session means it keeps working when you close your terminal, and you
can come back to it from anywhere.

```bash
limabean ssh                       # a shell in the VM's clone
tmux new -s agent                  # a session named agent
claude --dangerously-skip-permissions
```

Give Claude its task as you would on the Mac. To leave it working, detach
with **Ctrl-b d**; the session and Claude carry on. To come back:

```bash
limabean ssh
tmux attach -t agent
```

`--dangerously-skip-permissions` lets Claude run commands and edit files
without asking. Use it only in a VM like this one, locked and without
sudo, where the worst it can do is break a VM you can restore or throw
away. Leave the flag out to approve each step yourself.

To use Claude Desktop instead of a terminal, open the VM as an
[SSH connection](claude-desktop.md): Claude's tools all work in the VM.

## 2. One headless task

`claude -p` runs one prompt to the end without a session, prints the
result and exits. From the checkout on your Mac:

```bash
limabean ssh -- 'claude -p --dangerously-skip-permissions "make the failing test pass, and commit the fix on a new branch"'
```

It runs in the VM's clone, waits until Claude is done and exits with its
status, so it fits in a script or a Makefile target.

!!! note "Quoting"
    `limabean ssh -- …` joins the words after `--` and the VM's shell runs
    them, as `ssh` does. Put the whole command in single quotes, as above,
    and the prompt in double quotes inside it.

## 3. A long run in the background

For a task that takes hours, combine the two: a headless run inside a
detached tmux session, with its output in a log. Write the task in a file,
which spares you a second layer of quotes:

```bash
limabean ssh -- 'cat > ~/task.md' < TASK.md     # your prompt, from the Mac
limabean ssh -- 'tmux new -d -s agent "claude -p --dangerously-skip-permissions < ~/task.md 2>&1 | tee ~/agent.log"'
```

The command returns at once; the agent keeps going in the VM. Check on it
from the Mac:

```bash
limabean ssh -- tail -f ~/agent.log    # follow what it prints
limabean ssh -- tmux ls                # still running? the session is listed until it ends
limabean network log app --blocked     # anything it needed and couldn't reach
```

`claude -p` prints its answer when it finishes, so a quiet log means it's
still working; `git log` in the clone shows its commits as it goes. Its
session ends when Claude does, which is how you know it's done.

!!! note "Idle VMs stop by themselves"
    If you turned on [`limabean autostop`](lifecycle.md#idle-vms-stop-by-themselves),
    a VM with Claude Code running in it counts as in use, however quiet
    the run, so autostop leaves it alone until Claude exits.

## 4. A throwaway VM for one task

[`limabean run`](lifecycle.md#throwaway-vms) creates a VM, runs one
command in a fresh clone and, with `--rm`, deletes the VM afterwards. The
VM gets your shared secrets, so set `CLAUDE_CODE_OAUTH_TOKEN` without
`--vm` first, or pass it to this VM alone with
`--secret CLAUDE_CODE_OAUTH_TOKEN=<reference>`:

```bash
limabean run --rm --with claude --allow github --allow claude --locked \
  --repo me/app -- 'claude -p --dangerously-skip-permissions "fix issue 42, commit on a branch" && git push -u origin HEAD'
```

Nothing is left on your Mac or in a VM but the pushed branch. With
`--ttl 2h --name spike` instead of `--rm`, the VM stays for two hours, so
you can look around before it goes. A throwaway VM can't hand work to your
Mac with `limabean remote` once it's gone, which is why this one pushes.

## 5. Attempts side by side

[`limabean clone`](lifecycle.md#clones) copies a VM in seconds, `~/code`
and uncommitted work included, so several agents can start from the same
place:

```bash
limabean clone app app-b               # the source shuts down for a moment, then starts again
limabean clone app app-c
```

Give each one the task and start it. Outside the project's own VM, `ssh`
starts in the home directory, so `cd` to the clone first:

```bash
for vm in app-b app-c; do
  limabean ssh $vm -- 'cat > ~/task.md' < TASK.md
  limabean ssh $vm -- 'cd ~/code/github.com/me/app && tmux new -d -s agent "claude -p --dangerously-skip-permissions < ~/task.md 2>&1 | tee ~/agent.log"'
done
```

Give each VM a different `TASK.md` to run several tasks at once, or the
same one to compare attempts.

Each VM uses its share of your Mac's memory, so a few at a time is the
practical limit. Compare the results with a remote for each (below), keep
the best and delete the rest with `limabean rm app-b`.

## Getting the work out

Ask the agent to commit on a branch. Then either fetch it to your Mac
with [`limabean remote`](projects.md#review-on-your-mac), which works
whatever the VM's network allows:

```bash
limabean remote                              # the project's VM, as the remote "limabean"
limabean remote app-b --name app-b           # another VM, under its own name
git fetch --all
git diff main...app-b/fix-issue-42
```

or let the agent `git push` the branch itself, through your scoped
[GitHub token](github.md), and open a pull request from it.

To undo what an agent did to the VM, restore the snapshot from your Mac:
`limabean snapshot restore app`.

## What the sandbox does and doesn't do

- **The network is yours.** The agent can only reach what the VM's
  allowlist names. `limabean network log app` shows what it tried, and
  only you can allow more, with `limabean network allow` on the Mac.
- **Your Mac's files are out of reach.** The VM sees your git config and
  `~/.aws`, read-only, and nothing else.
- **Secrets are not hidden from it.** The agent can read every secret the
  VM has; they're environment variables. Locking the network limits where
  it could send them, so give agent VMs only the secrets they need, with
  `--vm`.
- **Your SSH agent is forwarded** while you're connected with
  `limabean ssh`, as in any VM. A GitHub token (`limabean github login`)
  lets an agent push without it, and works in detached sessions too.

## Next

- [Agent patterns](agent-patterns.md): worktrees, version matrices,
  learn-then-lock, a secret per role, writer and reviewer, checkpoints,
  Claude as the manager, scheduled agents and untrusted code.
- [Recipes](recipes.md): `claude-agent` and the others, as starting points.
- [Sandboxes](network.md): allowlists, learning what a VM needs, and the lock.
- [Rebuilds, clones and snapshots](lifecycle.md): the rest of what keeps work undoable.
