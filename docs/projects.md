# Projects: `.limabean.yaml`

A repository can describe the VM it needs in a `.limabean.yaml` file
(`.limabean.yml` works too) at its root: the tools, the size, what it may
reach, which secrets it uses and how to set it up. Then anyone with
limabean gets that VM, ready to work in, with one command from their
checkout:

```bash
cd ~/src/myapp
limabean up
```

## Your first project file

The quickest start is a [recipe](recipes.md), a commented starter file for
a kind of project (a Go service, a web frontend, a Rails app, a Claude
sandbox and more):

```bash
limabean recipes                 # list them
limabean recipes node-web        # write one to .limabean.yaml, then edit it
```

Or write one by hand. This is enough for a Node app that serves on port 3000:

```yaml
# .limabean.yaml
ports: [3000]
setup: |
  npm ci
```

Run `limabean up` anywhere in the checkout. It finds the file here or in a
parent directory, shows what it's about to do, and asks:

```
myapp (from ~/src/myapp/.limabean.yaml)
    environment: scratch
    clone: github.com/acme/myapp
    setup, run in the VM:
      npm ci
Create myapp? Yes
```

Then it:

1. creates a VM named after the directory (`myapp`), with the default
   [packs](packs.md) and your shared secrets;
2. clones the checkout's `origin` into the VM, at
   `~/code/github.com/acme/myapp`;
3. runs `setup` there, once;
4. prints how to reach it:

```
myapp is ready

ssh lima-myapp     or: limabean ssh myapp
limabean code myapp /home/you.linux/code/github.com/acme/myapp
http://localhost:3000
```

The VM is a normal limabean VM: `limabean ssh myapp`, `limabean code myapp`,
Claude Desktop and MCP all work with it. Commit the file so everyone on the
project gets the same VM.

## Every day

`limabean up` is safe to run any time. If the VM exists, it only starts it
(if it's stopped) and prints the same summary; nothing is created, cloned or
set up again.

```bash
limabean up                      # start it, wherever you are in the checkout
limabean ssh myapp               # a shell
limabean code myapp              # VS Code, Cursor or another editor, over SSH
limabean stop myapp              # or let autostop do it
```

## Changing the file

The file is read when `up` creates the VM. A VM made before a change keeps
what it had; to apply the change, make the VM again:

```bash
limabean rm myapp && limabean up
```

`rm` stops if a repository in the VM has unpushed work, so push first. The
new VM clones the repository again and runs `setup` from scratch. To try
other tools on the existing VM before changing the file, rebuild it with
them: `limabean rebuild myapp --with rust` keeps `~/code`.

## Reference

Every key is optional.

| Key | Type | Default | What it does |
|---|---|---|---|
| `name` | string | the directory's name | The VM's name: letters, digits and dashes |
| `env` | string | `scratch` | The [environment template](customizing.md) |
| `repo` | string | the checkout's `origin` | What to clone: `owner/repo`, `host/owner/repo` or a URL |
| `packs` | list | none | [Packs](packs.md) to add to the defaults (`PACKS`) |
| `without` | list | none | Default packs to leave out |
| `cpus` | number | `CPUS`, or 2 | CPUs |
| `memory` | number | `MEMORY`, or 4 | GiB of memory |
| `disk` | number | `DISK`, or 10 | GiB of system disk, which holds the tools and Docker images; `~/code` has a disk of its own |
| `network.allow` | list | open | Domains and presets the VM may reach ([sandboxes](network.md)) |
| `network.locked` | bool | `false` | Take sudo and Docker away from the VM's user, so the allowlist holds |
| `secrets` | list | none | Names of secrets the VM needs, besides the shared ones |
| `ports` | list | none | Ports the project serves, shown as `http://localhost:<port>` |
| `setup` | string | none | Shell commands run once in the clone, as you, with `bash -e` |
| `claude.marketplaces` | list | none | [Claude plugins](#claude-plugins) for the VM: each entry a `marketplace` (GitHub `owner/repo` or an https git URL) and the `plugins` to turn on from it, as `plugin@marketplace` |

Unknown keys, malformed names and out-of-range values are errors, so a typo
doesn't go unnoticed. A complete file:

```yaml
# .limabean.yaml
name: myapp
env: scratch
packs: [rust, claude]
without: [kubernetes]
cpus: 4
memory: 8
disk: 30
network:
  allow: [github, npm, rust]
  locked: true
secrets: [OPENAI_API_KEY, NPM_TOKEN]
ports: [3000, 5173]
setup: |
  npm ci
  cargo fetch
claude:
  marketplaces:
    - marketplace: rossedman/guardrails
      plugins: [guardrails@guardrails]
```

### Secrets

The file names secrets; the values stay on each person's Mac. Set a secret
up once, for this project's VM or for every VM, and `up` finds it:

```bash
limabean secrets set OPENAI_API_KEY op://Work/OpenAI/credential --vm myapp
limabean up
```

`up` gives the VM the secrets it names that are set up on your Mac, for any
VM, plus your shared secrets as always. It lists any it can't find, and
creates the VM without them; add them later with
`limabean secrets set NAME --vm myapp`, which installs them in the running
VM.

### Network

With `network.allow`, the VM can only reach those domains, and that already
applies when `setup` runs: allow whatever `setup` downloads from (`npm`,
`python`, `go`, `rust`…), and `github` for the clone. If you don't know the
list, find it out once:

```bash
limabean new trial --learn
limabean ssh trial 'cd ~/code && git clone https://github.com/acme/myapp && cd myapp && npm ci'
limabean network log trial          # everything it looked up
```

and put the domains in the file ([learn mode](network.md#learn-what-a-vm-needs)).

`locked: true` also applies before `setup`, so `setup` can't use `sudo` or
Docker. If it needs either, leave `locked` out and lock the VM afterwards
with `limabean network lock myapp`.

### Setup

`setup` runs with `bash -e`, so it stops at the first failing command. It
runs in the clone (in your home directory if nothing was cloned), as the
VM's user, with your secrets in the environment. If it fails, `up` stops and
keeps the VM so you can look: `limabean ssh myapp`. Fix the setup and start
over with `limabean rm myapp && limabean up`.

### Claude plugins

`claude` turns on plugins for Claude Code in the VM, whether it runs from
the [`claude` pack](packs.md#claude-code-in-a-vm) or from
[Claude Desktop](claude-desktop.md) over SSH. Skills, agents, hooks and
MCP servers all come inside plugins:

```yaml
claude:
  marketplaces:
    - marketplace: rossedman/guardrails                    # GitHub owner/repo
      plugins: [guardrails@guardrails]
    - marketplace: https://git.example.com/platform/plugins.git   # any git host, over https
      plugins: [review@platform, deploy@platform]
```

- Plugins are `plugin@marketplace`, where the marketplace is the name in
  its `marketplace.json` (often the repository's name). The plugins in one
  entry share it; another marketplace is another entry.
- `up` adds each marketplace to the VM's `~/.claude/settings.json` under
  `extraKnownMarketplaces`, and each plugin under `enabledPlugins`. Settings
  already there are kept. Claude Code adds the marketplace and installs the
  plugins when a session starts.
- Claude Code fetches them **inside the VM**, with the VM's own git
  credentials (a private repository works if
  [`limabean github login`](github.md) gave VMs access). A VM with a
  network allowlist needs `github`, or the marketplace's host.
- To change the list, edit the file, then
  `limabean rm myapp && limabean up`. To turn a plugin off in one VM, set it
  to `false` in that VM's `~/.claude/settings.json`.

A repository can also keep its own `.claude/settings.json` with
`enabledPlugins` and `extraKnownMarketplaces`; Claude Code reads that in
the clone after you trust the folder. Use `claude:` in `.limabean.yaml` for
what the VM should have whichever folder Claude opens, and for VMs that
Claude only uses headless.

## Examples

The [recipes](recipes.md) are complete examples, one per kind of VM: a Go
service with Postgres, a web frontend, Python APIs and notebooks, Rust,
Java, Rails, gRPC, Kubernetes, Terraform, a locked-down Claude Code
sandbox, a VM for reviewing untrusted code, and a minimal one. Start from
one with `limabean recipes <name>`.

## Trust

The file comes with the repository, and its setup runs in a VM that gets
your shared secrets. So `up` never acts on it blindly:

- In a terminal, it shows what the file asks for, setup commands included,
  and asks before creating anything.
- Without a terminal it shows the same and stops, unless you pass `--yes`.
- Claude plugins are fetched and run inside the VM, never on your Mac,
  and the plan lists every one of them.
- The file can only **name** secrets. It can't say where a value comes from,
  so it can't run a command on your Mac (as a `cmd:` reference would) or
  read a Keychain item you didn't already give VMs.

Read a file from someone else's repository before saying yes, as you would
a `Makefile`.

## Speed

The first VM of a kind (its environment and packs) is provisioned from
scratch, and an [image](lifecycle.md#fast-vms-images) of it is built in the
background. After that, `up` for any project with the same packs only has
to boot a copy. To have the image ready before the first `up`, build it:
`limabean images build --with rust,claude`.

## Commands

| Command | Does |
|---|---|
| `limabean up` | Creates the VM from `.limabean.yaml` (or `.yml`) here or in a parent directory, or starts it |
| `limabean up --yes` | The same without asking, for a file you trust |
| `limabean up --file <path>` | Uses another file |
