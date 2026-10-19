# Walkthrough: Claude fixes a bug in a sandbox

This page follows one task from start to finish: Claude Code, on your Mac,
takes a repository with a failing test, gives it a VM that can only reach
GitHub and npm, finds the bug, fixes it and shows you the change. Nothing
it runs touches your Mac, and you approve each step that changes something.

The example is `acme/shop`, a small Node app checked out at `~/src/shop`.
Use any repository of yours instead; the steps are the same.

You need a Mac with limabean, [Lima](getting-started.md) and
[Claude Code](https://code.claude.com). It takes about ten minutes, most of
it the first VM's provisioning.

## 1. Set up limabean and connect Claude

Once per Mac:

```bash
brew install rossedman/tap/limabean
limabean init      # settings, SSH config, then a check of everything

# limabean's tools for Claude Code
claude mcp add --scope user limabean -- limabean mcp
```

Start Claude Code in the repository, and check the tools are there with
`/mcp`:

```bash
cd ~/src/shop
claude
```

## 2. Let Claude draft the project file

The repository has no `.limabean.yaml` yet. Ask for one:

> Draft a .limabean.yaml for this repository. It should only reach GitHub
> and npm, and be locked.

Claude calls `project_init` with the checkout's path. limabean reads the
repository's files (here, `package.json` with Vite and a
`package-lock.json`) and returns a draft, without writing anything:

```yaml
# yaml-language-server: $schema=https://rossedman.github.io/limabean/limabean.schema.json
# Written by limabean init --project from what's in this repository.
# Check it, then: limabean up   (every key: limabean up --help)
packs: [node]
ports: [5173]
setup: |
  npm ci
```

Claude adds the network you asked for and writes the file with its own
editing tool, which asks you first, as any edit does:

```yaml
packs: [node]
ports: [5173]
network:
  allow: [github, npm]
  locked: true
setup: |
  npm ci
```

That's the whole description of the VM. `limabean init --project` does the
same draft in your terminal; [Projects](projects.md) covers every key.

## 3. Read the plan, then bring it up

> Bring up the VM for this checkout and run the tests.

Claude calls `project_plan` first and shows you what `up` would do,
changing nothing:

```
shop (from ~/src/shop/.limabean.yaml)
    env: scratch
    packs: core, node
    network: only github, npm, locked
    clone: github.com/acme/shop
    setup, run in the VM:
      npm ci
Nothing created (--preview)
```

Then it calls `project_up`. **Claude Code asks you before every call to
this tool**, whatever its permission mode, so you approve with the plan in
front of you. limabean then:

1. creates the VM `shop`, with Node from the `node` pack;
2. clones `acme/shop` into the VM at `~/code/github.com/acme/shop`, with
   your forwarded SSH agent or [GitHub token](github.md); your keys stay
   on the Mac;
3. turns the network allowlist on and locks it: from here on, the VM can
   reach GitHub and npm and nothing else, and has no sudo to change that;
4. runs `npm ci` in the clone, inside those limits, and sends its output
   back to Claude.

The first VM of a kind is provisioned from scratch and takes a few minutes;
later ones are [copies of an image](lifecycle.md#fast-vms-images).

## 4. Find the failure

Claude runs the tests in the VM with `run_in_vm`:

```
cd ~/code/github.com/acme/shop && npm test
```

and gets back the exit code and output: one failing test, say a cart
total that's off by one cent. It reads the failing test and the code under
test the same way, with commands in the VM (`cat`, `grep`, `git log`).

!!! note "Two copies of the code"
    The VM has its own clone of the repository; your `~/src/shop` on the
    Mac is a separate checkout. Claude Code's built-in file tools edit the
    **Mac's** copy, so in this setup ask Claude to work in the VM: it
    changes files there with commands, and runs the tests there too. For a
    long editing session inside the VM, open it as an
    [SSH connection in Claude Desktop](claude-desktop.md) instead, where
    every tool works in the VM.

## 5. Snapshot, then fix

> Snapshot the VM, then fix the bug in the VM's clone and run the tests
> until they pass. Show me the diff.

Claude calls `snapshot_save` first (the VM shuts down for a few seconds
and starts again), so any change can be rolled back. It then edits the file in the VM's clone,
runs `npm test` again until it passes, and shows you `git diff` from the
VM.

If the fix goes wrong, roll back from your terminal:

```bash
limabean snapshot restore shop
```

## 6. When the sandbox blocks something

Say a test downloads a fixture from `fixtures.example.com`. In this VM that
fails, because only GitHub and npm are allowed. Claude can see why with
`network_log`, which lists the names the VM looked up and whether each was
allowed:

```json
[{"name": "fixtures.example.com", "lookups": 3, "allowed": false}]
```

It tells you, and stops there: **no tool can widen a VM's network.** If
you agree, you allow it yourself:

```bash
limabean network allow shop fixtures.example.com
```

The same holds for the project file: a `.limabean.yaml` that later asks for
more domains isn't applied by `up`; it prints the command for you to run.
[Sandboxes](network.md) explains the allowlist and the lock.

## 7. Review and push, from the VM

The fix is in the VM's clone. Look at it and push it from there:

```bash
limabean code acme/shop          # opens the VM's clone in VS Code, Cursor or another editor
limabean ssh                     # or a shell in the VM
```

Inside the checkout, both pick the project's VM without a name. In the VM,
`git push` goes through your forwarded SSH agent (or your GitHub token), so
a branch can be pushed without your keys ever entering it; GitHub is on
this VM's allowlist.

## 8. Stop it, or throw it away

> Stop the VM.

Claude calls `project_down`; everything in the VM is kept for next time,
when `project_up` (or `limabean up` in the checkout) starts it again.
`limabean ps` shows its state from your terminal.

To delete it, run `limabean rm shop`. It checks for unpushed work first,
and asks. Claude can't delete VMs unless you start the server with
[`--allow-destructive`](mcp.md#tools-that-destroy-work), and even then
Claude Code asks you each time.

## What kept your Mac out of it

- **Every command ran in the VM.** No tool runs anything on the Mac;
  `project_init` only reads the checkout, and only a git checkout.
- **You approved the changes.** `project_up` asks every time, and Claude's
  edit to `.limabean.yaml` went through Claude Code's own approval.
- **The VM could reach two services**, and nothing in it, Claude included,
  could change that.
- **Your keys and secrets stayed on the Mac.** Pushes used your SSH agent
  or a scoped token;
  secret values, had the project named any, go to the VM over stdin and
  never into Claude's view.
- **A snapshot** made any step undoable.

## Next

- [Claude and MCP](mcp.md): setting up Claude Desktop, every tool, and the
  destructive ones.
- [Projects](projects.md): every key of `.limabean.yaml`, and editor
  support.
- [The `claude-agent` recipe](recipes.md): a locked VM with Claude Code
  inside it, for agents that work on their own.
