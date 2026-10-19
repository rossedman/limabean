# Agent patterns

[Agents in a VM](agents.md) shows the basics: Claude Code inside a locked
VM, in tmux, headless and in throwaway VMs. This page builds on it with
patterns for running several agents, fitting each one's sandbox to its
job, having agents check each other's work, and running them while you're
away. Each section says when to use it, gives the commands, and ends with
what to watch.

The examples use a project whose VM is `app` and whose repository is
`me/app`, cloned in each VM to `~/code/github.com/me/app`.

## Several agents in one VM, with worktrees

**When:** a few independent tasks on the same repository, without the
memory of a VM each.

`git worktree` gives each task its own checkout of the same repository,
on its own branch, so agents don't edit each other's files. Put the
worktrees under `~/code`, which [survives rebuilds](lifecycle.md#rebuilds-keep-code):

```bash
limabean ssh                       # a shell in the clone
git worktree add ~/code/worktrees/app-docs -b agent/docs
git worktree add ~/code/worktrees/app-perf -b agent/perf

tmux new -d -s docs -c ~/code/worktrees/app-docs 'claude -p --dangerously-skip-permissions "update the README for the new CLI flags; commit" 2>&1 | tee ~/docs.log'
tmux new -d -s perf -c ~/code/worktrees/app-perf 'claude -p --dangerously-skip-permissions "find and fix the slowest test; commit" 2>&1 | tee ~/perf.log'
```

`tmux ls` lists the agents still running. Every worktree shares one
repository, so a single [`limabean remote`](projects.md#review-on-your-mac)
and `git fetch limabean` on the Mac bring back all their branches.

**Watch:** they share the VM's CPU, memory and network, so builds slow each
other down. `limabean rm` checks for unpushed commits on every branch,
worktrees' included, but not for uncommitted changes inside a worktree:
have the agents commit, or remove worktrees with `git worktree remove`,
before deleting the VM.

## The same task against different versions

**When:** checking a change against several versions of a tool, such as
two Go releases, or letting attempts differ in one way you choose.

Give each VM its own project file, naming the VM and pinning the
[tool](projects.md#tools-and-apt) that differs:

```yaml
# matrix/go125.yaml
name: app-go125
repo: me/app
packs: [go, claude]
tools:
  go: "1.25.4"
network:
  allow: [github, go, claude]
  locked: true
```

```bash
limabean up --file matrix/go125.yaml --yes
limabean up --file matrix/go126.yaml --yes
```

Then start the same task in each, as in
[attempts side by side](agents.md#5-attempts-side-by-side), and fetch the
branches with a remote per VM.

**Watch:** each VM installs its own project tools, so these VMs take longer
to create than ones that only use packs.

## Learn what an agent needs, then lock it

**When:** a new project, where you don't yet know which hosts its build
and tests reach.

Let the first run happen in learning mode, which keeps the network open
but records every name the VM looks up. Then turn what was used into the
allowlist and lock it:

```bash
limabean network learn app
limabean ssh -- 'claude -p --dangerously-skip-permissions "install the dependencies and get the test suite passing"'
limabean network log app               # every name it looked up, and how often
limabean network learn app --done      # allow just those from now on
limabean network lock app
```

Later runs can reach only what the first one needed. Put the result in
the project file's `network:` so new VMs start locked
([Sandboxes](network.md#learn-what-a-vm-needs)).

**Watch:** during learning the agent can reach anything. Do it with a task
you'd trust on an open network, and read the list `--done` prints before
locking: a lookup made once by something unexpected is allowed too.

## A secret for each role

**When:** agents with different jobs, such as one that pushes and one that
only reads.

Shared secrets go to every VM, and so does the token from
`limabean github login`. Keep the shared file to what every VM may have,
and give tokens to the VMs that need them with `--vm`:

```bash
limabean secrets set GITHUB_TOKEN keychain:gh-app-writer --vm app        # a fine-grained token that can push to me/app
limabean secrets set CLAUDE_CODE_OAUTH_TOKEN keychain:claude --vm app
limabean secrets set CLAUDE_CODE_OAUTH_TOKEN keychain:claude --vm reviewer
limabean secrets ls                    # which VM gets what
```

A reviewer then has a Claude login and nothing that can push, and a VM for
code you don't trust gets nothing at all.

**Watch:** an agent can read every secret in its VM. A
[fine-grained GitHub token](github.md) limited to one repository limits
what a leaked one can do.

## A writer and a reviewer

**When:** you want a second opinion before reading the change yourself,
from an agent that can't change anything.

One VM writes a branch; a second, locked VM with no push rights reviews
it. Your Mac carries the branch between them, so neither VM needs to
reach the other, or GitHub, for it:

```bash
# A reviewer: Claude and the supply-chain scanners, no push token
limabean up reviewer --with claude,supply-chain --allow github --allow claude --locked
limabean get reviewer me/app

# In the checkout on the Mac: the writer's branch, from one VM to the other
limabean remote                              # the writer, as "limabean"
limabean remote reviewer --name reviewer     # the reviewer's clone
git fetch limabean
git push reviewer limabean/agent/fix:refs/heads/agent/fix

limabean ssh reviewer -- 'cd ~/code/github.com/me/app && git diff main...agent/fix | claude -p "Review this change. List bugs, risks and missing tests, most serious first. Change nothing."'
```

The reviewer reads the diff from its input, so it needs no permission to
run commands. The [`review` recipe](recipes.md) is a project file for the
same kind of VM, with Claude's code-review plugin turned on.

**Watch:** the review is advice. It catches things, but it isn't a reason
to skip reading the change yourself.

## Checkpoints for a long task

**When:** a task with phases, where a failure late on shouldn't cost the
early work.

Save a snapshot between phases, and roll back to the last good one rather
than starting again:

```bash
limabean snapshot save app after-setup
# … the agent refactors …
limabean snapshot save app after-refactor
# … the migration goes wrong …
limabean snapshot restore app after-refactor
limabean snapshot ls app
```

A snapshot holds the VM's system and `~/code`, uncommitted work included
([Snapshots](lifecycle.md#snapshots)).

**Watch:** saving shuts the VM down for a few seconds, which ends a running
agent and its tmux session. Save between runs, not during one.

## Claude on the Mac as the manager

**When:** you want one conversation to run several agents and report back,
rather than juggling terminals.

Claude Code on your Mac can do the steps of
[attempts side by side](agents.md#5-attempts-side-by-side) itself, over
[MCP](mcp.md): `clone_vm` to make the copies, `run_in_vm` to start a tmux
agent in each and later to read their logs, and `vm_info` to see each
VM's clone and network. Ask it for the whole job:

> Clone app twice. In each copy, start Claude in tmux on the task in
> ~/task.md. Check their logs every few minutes, and when both are done,
> compare their branches and tell me which to keep.

**Watch:** Claude Code asks before each tool you haven't allowed, and
`run_in_vm` can run anything in a VM, so read what it starts. It can't
widen a VM's network or delete one; you do that.

## Watch the app while the agent works

**When:** front-end or API work, where seeing the result beats reading the
diff.

A project file's [`start`](projects.md#start) runs the dev server in the
background, and its port reaches your Mac's `localhost`:

```yaml
start: npm run dev
ports: [3000]
```

```bash
limabean up
limabean open app 3000                 # http://localhost:3000, in your browser
```

As the agent edits files in the clone, the dev server reloads and you see
each change. `limabean logs --start -f` follows the server's output.

**Watch:** `start` runs commands from the project file on every `up`, so
limabean asks you to approve them the first time and whenever they change.

## Agents on a schedule

**When:** chores that suit a quiet hour, such as updating dependencies
overnight and opening a pull request.

A launchd agent on your Mac can run [`limabean run`](lifecycle.md#throwaway-vms)
on a schedule. Save this as
`~/Library/LaunchAgents/com.example.limabean-deps.plist`, with your
repository and task:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.example.limabean-deps</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/limabean</string>
    <string>run</string><string>--rm</string><string>--plain</string>
    <string>--with</string><string>claude</string>
    <string>--allow</string><string>github</string>
    <string>--allow</string><string>npm</string>
    <string>--allow</string><string>claude</string>
    <string>--locked</string>
    <string>--repo</string><string>me/app</string>
    <string>--</string>
    <string>claude -p --dangerously-skip-permissions "update the npm dependencies within their major versions, run the tests, and if they pass push a branch and open a pull request with gh" </string>
  </array>
  <key>EnvironmentVariables</key>
  <dict><key>PATH</key><string>/opt/homebrew/bin:/usr/bin:/bin</string></dict>
  <key>StartCalendarInterval</key>
  <dict><key>Hour</key><integer>3</integer><key>Minute</key><integer>17</integer></dict>
  <key>StandardOutPath</key><string>/tmp/limabean-deps.log</string>
  <key>StandardErrorPath</key><string>/tmp/limabean-deps.log</string>
</dict>
</plist>
```

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.limabean-deps.plist
```

The VM gets your shared secrets. To keep the Claude login and the GitHub
token out of the shared file, pass them to this VM alone with `run`'s
`--secret NAME=reference` (for example a `keychain:` or `op://`
[reference](secrets.md)).

**Watch:** launchd runs it only while you're logged in and the Mac is
awake. The pull request is the place to review the agent's work; don't let
a scheduled agent merge.

## Run code you don't trust, and keep the evidence

**When:** a script, a package or a repository you want to see run before
you trust it.

Run it in a locked VM that deletes itself, then read what it tried to
reach:

```bash
limabean run --ttl 1h --name probe --allow github --locked \
  --repo someone/tool -- 'make install && ./tool --help'
limabean network log probe --blocked   # every host it tried and was refused
```

`--ttl` keeps the VM for an hour, so you can look around in it before it
goes. The `run_untrusted` [workflow](mcp.md#ready-made-workflows) does the
same through Claude, and reads the code with you first.

**Watch:** a new VM gets your shared secrets. For this, keep the shared
file empty, or check `limabean secrets ls` first.
