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
packs: [node]
ports: [3000]
setup: |
  npm ci
```

Run `limabean up` anywhere in the checkout. It finds the file here or in a
parent directory, shows what it's about to do, and asks:

```
myapp (from ~/src/myapp/.limabean.yaml)
    environment: scratch
    packs: core, node
    clone: github.com/acme/myapp
    setup, run in the VM:
      npm ci
Create myapp? Yes
```

Then it:

1. creates a VM named after the directory (`myapp`), with the
   [packs](packs.md) the file lists and your shared secrets;
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

Like `docker compose`, `up` and `down` work from the checkout, wherever you
are in it:

```bash
limabean up                      # create the VM, or start it and apply the file's changes
limabean ssh myapp               # a shell
limabean code myapp              # VS Code, Cursor or another editor, over SSH
limabean ps                      # is it running, is start, which ports, what changed
limabean logs --start -f         # follow what start is running
limabean down                    # stop it; everything in it is kept
```

`down` only stops the VM. `limabean rm myapp` deletes it.

## Changing the file

Edit the file and run `limabean up` again. It compares the file with the VM
as it is, lists what differs and asks before changing anything (`--yes`
skips the question; `--dry-run` only lists it):

```
myapp (changes from ~/src/myapp/.limabean.yaml)
    packs: core, node → core, node, rust (rebuild, keeping ~/code)
    cpus: 2 → 4
    network: stop allowing yarnpkg.com
Apply these changes to myapp? Yes
```

| Change | What `up` does |
|---|---|
| `packs`, `env` | Rebuilds the VM, as [`limabean rebuild`](lifecycle.md) does: `~/code` and its repositories are kept on their own disk, the rest of the system is fresh |
| `cpus`, `memory` | Stops the VM, resizes it and starts it again |
| `disk` | Grows the disk the same way; Lima can't shrink one, so a smaller number is only a warning |
| `secrets` | Gives the VM the secrets it doesn't have yet |
| `environment` | Writes the new variables in the running VM |
| `claude` | Adds the plugins to the VM's Claude settings |
| `network`: fewer domains, `locked: true` | Applied straight away |
| `network`: more domains, no allowlist, no `locked` | **Not applied.** A file from a repository can't widen what a VM reaches; `up` prints the `limabean network …` command for you to run if you agree |
| `setup` | Runs only when the VM is created; `limabean up --setup` runs it again in the clone |
| `start` | Shows the new commands and asks; once approved, they run after this and every later `up` |
| `name` | Makes a different VM; the old one stays until you `limabean rm` it |

A VM without its own `~/code` disk (made with `CODE_DISK=""`) loses its
repositories in a rebuild, so `up` refuses one while any of them has
unpushed work.

## Reference

Every key is optional.

| Key | Type | Default | What it does |
|---|---|---|---|
| `name` | string | the directory's name | The VM's name: letters, digits and dashes |
| `env` | string | `scratch` | The [environment template](customizing.md) |
| `repo` | string | the checkout's `origin` | What to clone: `owner/repo`, `host/owner/repo` or a URL |
| `packs` | list | none | The VM's [packs](packs.md) besides `core`; the `PACKS` setting doesn't apply, so everyone gets the same VM |
| `cpus` | number | `CPUS`, or 2 | CPUs |
| `memory` | number | `MEMORY`, or 4 | GiB of memory |
| `disk` | number | `DISK`, or 10 | GiB of system disk, which holds the tools and Docker images; `~/code` has a disk of its own |
| `network.allow` | list | open | Domains and presets the VM may reach ([sandboxes](network.md)) |
| `network.locked` | bool | `false` | Take sudo and Docker away from the VM's user, so the allowlist holds |
| `environment` | map | none | Plain variables for the VM's shells, `setup` and `start`; [details](#environment) |
| `secrets` | list | none | Names of secrets the VM needs, besides the shared ones |
| `ports` | list | none | Ports the project serves, shown as `http://localhost:<port>` |
| `setup` | string | none | Shell commands run once in the clone, as you, with `bash -e` |
| `start` | string | none | Shell commands run in the clone, [in the background](#start), after every `up` |
| `claude.marketplaces` | list | none | [Claude plugins](#claude-plugins) for the VM: each entry a `marketplace` (GitHub `owner/repo` or an https git URL) and the `plugins` to turn on from it, as `plugin@marketplace` |

Unknown keys, malformed names and out-of-range values are errors, so a typo
doesn't go unnoticed. A complete file:

```yaml
# .limabean.yaml
name: myapp
env: scratch
packs: [node, rust, claude]
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

### Environment

`environment` sets plain variables in the VM, for its shells, `setup` and
`start`:

```yaml
environment:
  NODE_ENV: development
  DATABASE_URL: postgres://postgres@localhost:5432/app
```

- They're in the file, and so in the repository: anything secret goes
  under [`secrets`](#secrets) instead, which only names it.
- `up` writes them to `~/.config/limabean/project.env` in the VM, which
  every shell reads. A change to them is applied to the running VM, no
  rebuild needed; shells already open see it in their next login.
- A project file can't set `PATH`, `HOME`, `SHELL` and the like, or the
  dynamic linker's `LD_*` variables, and a name can't be both a variable
  and a secret. Secrets are read after these, so they always win.

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
limabean up trial --learn
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
keeps the VM so you can look: `limabean ssh myapp`, or read what it printed
with `limabean logs --setup`. Fix the setup, then run it again with
`limabean up --setup`.

### Start

`start` is for what should be running whenever the VM is up: services in
Docker, a dev server, a watcher. It runs after every `up`, in the clone,
with `bash -e` and your secrets, in the background, so `up` returns while
it keeps going:

```yaml
start: |
  docker compose up -d
  npm run dev -- --host 0.0.0.0
```

- If the last `start` is still running, `up` leaves it alone rather than
  starting a second one.
- It stops with the VM: `limabean down`, and it runs again at the next `up`.
- `limabean logs --start -f` follows its output; `start` keeps only the
  latest run's.
- `start` commands come with the repository and run on every `up`, so `up`
  remembers the ones you approved (on your Mac) and asks again whenever they
  change, as after a `git pull` that edits them. Until you approve, the old
  ones don't run either.

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
- To add plugins, edit the file and run `limabean up`. To turn one off in a
  VM, set it to `false` in that VM's `~/.claude/settings.json`; removing it
  from the file doesn't turn it off.

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
