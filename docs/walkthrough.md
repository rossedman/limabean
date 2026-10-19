# Walkthrough: Claude makes a change in a sandbox

This page follows one real task from start to finish, on a real repository:
limabean's own. Claude Code, on your Mac, gives the repository a VM that can
only reach GitHub and Go's module servers, adds a pack for Zig the way
[CONTRIBUTING.md](https://github.com/rossedman/limabean/blob/main/CONTRIBUTING.md)
describes, finds the test that breaks, fixes it and shows you the change.
Nothing it runs touches your Mac, and you approve each step that changes
something.

The output on this page is what limabean prints for this repository. Use any
repository of yours instead; the steps are the same.

You need a Mac with limabean, [Lima](getting-started.md) and
[Claude Code](https://code.claude.com). It takes about fifteen minutes, most
of it the first VM's provisioning.

## 1. Set up limabean and connect Claude

Once per Mac:

```bash
brew install rossedman/tap/limabean
limabean init      # settings, SSH config, then a check of everything

# limabean's tools for Claude Code
claude mcp add --scope user limabean -- limabean mcp
```

Then clone limabean, and start Claude Code in it. `/mcp` lists limabean's
tools:

```bash
git clone https://github.com/rossedman/limabean ~/src/limabean
cd ~/src/limabean
claude
```

## 2. Let Claude draft the project file

limabean's repository has no `.limabean.yaml`. Ask for one:

> Draft a .limabean.yaml for this repository. It should only reach GitHub
> and Go's module servers, and be locked.

Claude calls `project_init` with the checkout's path. limabean reads the
repository's files and returns a draft, writing nothing. It found `go.mod`,
and `docs/requirements.txt` for the docs site:

```yaml
# yaml-language-server: $schema=https://rossedman.github.io/limabean/limabean.schema.json
# Written by limabean init --project from what's in this repository.
# Check it, then: limabean up   (every key: limabean up --help)
packs: [go, python]
setup: |
  go mod download
  (cd docs && uv venv && uv pip install -r requirements.txt)
```

Claude adds the network you asked for (the `github` and `go` presets) and
writes the file with its own editing tool, which asks you first, as any edit
does:

```yaml
packs: [go, python]
network:
  allow: [github, go]
  locked: true
setup: |
  go mod download
  (cd docs && uv venv && uv pip install -r requirements.txt)
```

The file is only on your Mac; you don't have to commit it.
`limabean init --project` makes the same draft in your terminal, and
[Projects](projects.md) covers every key.

## 3. Read the plan, then bring it up

> Bring up the VM for this checkout.

Claude calls `project_plan` first and shows you what `up` would do,
changing nothing. The VM is named after the directory:

```
limabean (from ~/src/limabean/.limabean.yaml)
    env: scratch
    packs: core, go, python
    network: only github, go, locked
    clone: github.com/rossedman/limabean
    setup, run in the VM:
      go mod download
      (cd docs && uv venv && uv pip install -r requirements.txt)
Nothing created (--preview)
```

Then it calls `project_up`. **Claude Code asks you before every call to
this tool**, whatever its permission mode, so you approve with the plan in
front of you. limabean then:

1. creates the VM `limabean`, with Go and uv from the packs;
2. clones `rossedman/limabean` into the VM at
   `~/code/github.com/rossedman/limabean`;
3. turns the allowlist on and locks it: from here on, the VM can reach
   GitHub and Go's module servers and nothing else, and has no sudo to
   change that;
4. runs the setup in the clone, inside those limits, and sends its output
   back to Claude.

The first VM of a kind is provisioned from scratch and takes a few minutes;
later ones are [copies of an image](lifecycle.md#fast-vms-images).

## 4. The sandbox blocks something

`go mod download` works. The second setup line doesn't: it installs the
docs site's Python packages from PyPI, and PyPI isn't on the allowlist.
`project_up` reports `setup failed`, with the output; the VM is up and
locked, and `go mod download` has run. Claude can confirm
why with `network_log`, which lists the names the VM looked up and whether
each was allowed:

```json
[{"name": "pypi.org", "lookups": 2, "allowed": false}]
```

It tells you, and stops there: **no tool can widen a VM's network**, and
neither can the project file once the VM exists. You decide. The docs
packages aren't needed for the tests, so you can leave it blocked; or allow
PyPI yourself:

```bash
limabean network allow limabean python
limabean up --setup              # runs the setup again, in the clone
```
[Sandboxes](network.md) explains the allowlist and the lock.

## 5. Make the change

> Add a pack for Zig, with zig and zls, following CONTRIBUTING.md. Work in
> the VM's clone, run the tests there, and show me the diff.

!!! note "Two copies of the code"
    The VM has its own clone of the repository; `~/src/limabean` on your Mac
    is a separate checkout. Claude Code's built-in file tools edit the
    **Mac's** copy, so this prompt asks Claude to work in the VM: it changes
    files there with commands, and runs the tests there too. For a long
    editing session inside the VM, open it as an
    [SSH connection in Claude Desktop](claude-desktop.md) instead, where
    every tool works in the VM.

Claude calls `snapshot_save` first (the VM shuts down for a few seconds and
starts again), so the change can be rolled back. Then, with `run_in_vm`, it
writes `templates/packs/zig.toml` in the clone:

```toml
# Zig and its language server.
[tools]
zig = "0.16.0"
zls = "0.16.0"   # zls follows zig's minor version
```

adds a row for it to `docs/packs.md`, as CONTRIBUTING.md asks, and runs the
tests:

```
cd ~/code/github.com/rossedman/limabean && go test ./...
```

One fails:

```
--- FAIL: TestSchemaIsCurrent (0.00s)
    schema_test.go:62: docs/limabean.schema.json is out of date: go test ./internal/cli -run TestSchema -update-schema
FAIL
FAIL	github.com/rossedman/limabean/internal/cli	6.263s
```

The project file's schema lists the built-in packs, so a new pack changes
it. The failure says how to fix that; Claude runs it, then the tests again:

```
go test ./internal/cli -run TestSchema -update-schema
go test ./...
ok  	github.com/rossedman/limabean/internal/cli	6.250s
```

and shows you the change, with `git diff --stat` and `git diff`:

```
 docs/limabean.schema.json | 3 ++-
 docs/packs.md             | 1 +
 templates/packs/zig.toml  | 4 ++++
```

If anything had gone wrong, you'd roll back from your terminal:

```bash
limabean snapshot restore limabean
```

## 6. Review and push, from the VM

The change is in the VM's clone. Look at it and push a branch from there:

```bash
limabean code rossedman/limabean   # opens the VM's clone in VS Code, Cursor or another editor
limabean ssh                       # or a shell in the VM
```

Inside the checkout, both pick the project's VM without a name. In the VM,
`git push` goes through your forwarded SSH agent (or your
[GitHub token](github.md)), so a branch can be pushed without your keys
ever entering it; GitHub is on this VM's allowlist.

## 7. Stop it, or throw it away

> Stop the VM.

Claude calls `project_down`; everything in the VM is kept for next time,
when `project_up` (or `limabean up` in the checkout) starts it again.
`limabean ps` shows its state from your terminal.

To delete it, run `limabean rm limabean`. It checks for unpushed work
first, and asks. Claude can't delete VMs unless you start the server with
[`--allow-destructive`](mcp.md#tools-that-destroy-work), and even then
Claude Code asks you each time.

## What kept your Mac out of it

- **Every command ran in the VM.** No tool runs anything on the Mac;
  `project_init` only reads the checkout, and only a git checkout.
- **You approved the changes.** `project_up` asks every time, and Claude's
  edit to `.limabean.yaml` went through Claude Code's own approval.
- **The VM could reach two services.** When setup wanted a third, it was
  blocked, and only you could allow it.
- **Your keys stayed on the Mac.** The clone and the push used your SSH
  agent or a scoped token, and secret values, had the project named any,
  go to the VM over stdin and never into Claude's view.
- **A snapshot** made the change undoable.

## Next

- [Claude and MCP](mcp.md): setting up Claude Desktop, every tool, and the
  destructive ones.
- [Projects](projects.md): every key of `.limabean.yaml`, and editor
  support.
- [The `claude-agent` recipe](recipes.md): a locked VM with Claude Code
  inside it, for agents that work on their own.
