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

The quickest start is to let limabean write it from what's in the
repository:

```bash
cd ~/src/myapp
limabean init --project          # shows what it found, asks, writes .limabean.yaml
limabean init --project --print  # or just show it
```

It looks at the repository's root and the directories directly below it,
so a monorepo's `web/` and `api/` count too:

| It finds | It writes |
|---|---|
| `go.mod`, `package.json`, `pyproject.toml`, `requirements.txt`, `Cargo.toml`, `Gemfile`, `mix.exs`, `pom.xml`, `build.gradle`, a `.csproj` | the [pack](packs.md), and a `setup` command that fits the lockfile: `npm ci`, `pnpm install --frozen-lockfile`, `uv sync`, `go mod download`, `bundle install` and so on |
| `*.tf`, `Chart.yaml`, `kustomization.yaml`, `skaffold.yaml` | the `infra` or `kubernetes` pack |
| Vite, Next.js, Astro, Rails, Django, FastAPI, Flask, Phoenix or Spring Boot | the port it serves on |
| `compose.yaml` | `start: docker compose up -d` |
| `.tool-versions`, `mise.toml` | the pack with that tool, or the tool under [`tools`](#tools-and-apt) if no pack has it |
| `.env.example` | its credentials (`*_KEY`, `*_TOKEN`, …) as a commented-out `secrets:` line to set up first |

It only reads files, never runs anything from the repository, and doesn't
follow symlinks out of it. The file it writes passes the same checks as
`up`; read it before you bring the VM up, since a guess can be wrong.

Or start from a [recipe](recipes.md), a commented starter file for a kind
of project (a Go service, a web frontend, a Rails app, a Claude sandbox
and more):

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
    env: scratch
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

`limabean doctor` in the checkout checks the file against this Mac: that
its packs, network presets and secrets exist here, and whether the VM
matches it.

### In your editor

Your editor can complete the file's keys and flag mistakes as you type: see
[Editor support](#editor-support). Files written by `limabean recipes`
already have the line that turns it on.

## Every day

Like `docker compose`, commands work from the checkout, wherever you are
in it, without naming the VM:

```bash
limabean up                      # create the VM, or start it and apply the file's changes
limabean ssh                     # a shell, in the VM's clone of the repository
limabean ssh -- make test        # a command, run there
limabean code                    # VS Code, Cursor or another editor, over SSH
limabean ps                      # is it running, is start, which ports, what changed
limabean logs --start -f         # follow what start is running
limabean down                    # stop it; everything in it is kept
```

`ssh`, `code`, `get`, `open`, `ports`, `network ls`, `snapshot save` and
`snapshot ls` use the project's VM when you don't name one; name another
VM to use it instead. `rm`, `rebuild` and `snapshot restore` always need
the name, so nothing is deleted or rolled back by accident.

In the checkout, `ssh` works in the VM's clone (`~/code/<host>/<owner>/<repo>`)
rather than the home directory, so `limabean ssh -- go test ./...` runs the
tests there. The clone is separate from your checkout on the Mac: changes
made in one stay there until they're pushed.

In the VM, [`limabean-info`](inside-a-vm.md#what-this-vm-is) shows the
project's repository, clone, ports and logs, which `up` writes there.

`down` only stops the VM. `limabean rm myapp` deletes it.

## Changing the file

Edit the file and run `limabean up` again. It compares the file with the VM
as it is, lists what differs and asks before changing anything (`--yes`
skips the question; `--preview` only lists it):

```
myapp (changes from ~/src/myapp/.limabean.yaml)
    packs: core, node → core, node, rust (rebuild, keeping ~/code)
    cpus: 2 → 4
    network: stop allowing yarnpkg.com
Apply these changes to myapp? Yes
```

| Change | What `up` does |
|---|---|
| `packs`, `tools`, `apt`, `env` | Rebuilds the VM, as [`limabean rebuild`](lifecycle.md) does: `~/code` and its repositories are kept on their own disk, the rest of the system is fresh |
| `cpus`, `memory` | Stops the VM, resizes it and starts it again |
| `disk` | Grows the disk the same way; Lima can't shrink one, so a smaller number is only a warning |
| `secrets` | Gives the VM the secrets it doesn't have yet |
| `vars` | Writes the new variables in the running VM |
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
| `tools` | map | none | The project's own [mise tools](#tools-and-apt), `tool: version`, for what no pack has |
| `apt` | list | none | The project's own [Ubuntu packages](#tools-and-apt) |
| `cpus` | number | `CPUS`, or 2 | CPUs |
| `memory` | number | `MEMORY`, or 4 | GiB of memory |
| `disk` | number | `DISK`, or 10 | GiB of system disk, which holds the tools and Docker images; `~/code` has a disk of its own |
| `network.allow` | list | open | Domains and presets the VM may reach ([sandboxes](network.md)) |
| `network.locked` | bool | `false` | Take sudo and Docker away from the VM's user, so the allowlist holds |
| `vars` | map | none | Plain variables for the VM's shells, `setup` and `start`; [details](#vars) |
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

### Tools and apt

When a project needs a tool or a system package that no [pack](packs.md)
has, list it in the file instead of making a pack on every Mac:

```yaml
tools:                        # mise tools, tool: version
  terraform: 1.9.8
  "npm:@biomejs/biome": 2.1.2 # any mise backend: aqua:, npm:, pipx:, cargo:, go:
apt: [libpq-dev, graphviz]    # Ubuntu packages
```

- They're installed in the VM when it's created, the same way packs are,
  so a VM [copied from an image](lifecycle.md#fast-vms-images) gets them
  too. A change to them rebuilds the VM, keeping `~/code`, as a change to
  `packs` does; `limabean rebuild` keeps them.
- Pin exact versions, as packs do, so everyone gets the same tools.
  `limabean bump` doesn't update these; edit the file.
- A tool a pack already pins is best left to the pack.
- Like packs, they're installed at the VM's first boot, before its
  [network policy](#network) applies, so a `locked` VM still gets them.
- Names and versions are checked: only letters, digits and `._@/:+-`
  for tools, Debian package names for `apt`.

### Vars

`vars` sets plain variables in the VM, for its shells, `setup` and
`start`:

```yaml
vars:
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

## Editor support

limabean publishes a [JSON Schema](limabean.schema.json) for the project
file. With it, your editor:

- completes keys, and offers the built-in [packs](packs.md) and
  [network presets](network.md) as values;
- shows what each key does when you hover over it;
- flags what `up` would refuse while you type: an unknown key such as
  `enviroment:`, a misspelled or reserved variable (`PATH`), a port out of
  range, a malformed tool version or package name, a marketplace without
  plugins.

The schema is generated from the same checks `limabean up` runs, so the two
agree. A few things only `up` can check, because they depend on your Mac:
that a secret is set up, that a pack of your own exists (the schema accepts
any pack name, so yours aren't flagged), and that a repository can be
cloned.

### One file: the comment line

Editors that use the YAML language server read a comment at the top of the
file. `limabean recipes` writes it for you; add it to a file you wrote by
hand:

```yaml
# yaml-language-server: $schema=https://rossedman.io/limabean/limabean.schema.json
packs: [node]
```

That works in VS Code and Cursor (with the
[YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml)),
Zed, Neovim with `yamlls`, Helix, and any other editor running the YAML
language server. Committed with the file, it works for everyone on the
project.

### Every project: editor settings

To get it in every `.limabean.yaml` without the comment, map the schema to
the file name in your editor's settings.

**VS Code and Cursor**: install the
[YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml),
then add to your `settings.json`:

```json
"yaml.schemas": {
  "https://rossedman.io/limabean/limabean.schema.json": ["**/.limabean.yaml", "**/.limabean.yml"]
}
```

**Zed**: in `settings.json`:

```json
"lsp": {
  "yaml-language-server": {
    "settings": {
      "yaml": {
        "schemas": {
          "https://rossedman.io/limabean/limabean.schema.json": ["**/.limabean.yaml", "**/.limabean.yml"]
        }
      }
    }
  }
}
```

**Neovim** with [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig)
and `yaml-language-server` installed:

```lua
require("lspconfig").yamlls.setup({
  settings = {
    yaml = {
      schemas = {
        ["https://rossedman.io/limabean/limabean.schema.json"] = { "**/.limabean.yaml", "**/.limabean.yml" },
      },
    },
  },
})
```

**Helix**: in `languages.toml`:

```toml
[language-server.yaml-language-server.config.yaml.schemas]
"https://rossedman.io/limabean/limabean.schema.json" = ["**/.limabean.yaml", "**/.limabean.yml"]
```

**JetBrains IDEs** (GoLand, IntelliJ, PyCharm and the rest): open
**Settings → Languages & Frameworks → Schemas and DTDs → JSON Schema
Mappings**, add a mapping with the schema's URL, and add the file patterns
`.limabean.yaml` and `.limabean.yml`.

### When it doesn't work

- **Nothing is completed or flagged.** Check that the YAML language server
  is running for the file: in VS Code and Cursor, the YAML extension must be
  installed, and the language mode in the status bar must say YAML.
- **The editor can't fetch the schema.** It downloads it from the docs site.
  Offline, or behind a proxy that blocks it, save
  [the schema](limabean.schema.json) next to your project and point the
  comment or the mapping at the file's path instead of the URL.
- **A pack of yours is underlined.** It shouldn't be: any pack name is
  accepted. If your editor shows one anyway, it has an old copy of the
  schema; restarting the language server, or the editor, refreshes it.

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
