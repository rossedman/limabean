## limabean up

Create or start a VM, or bring a project's VM up to date

### Synopsis

Bring up a VM: up is how VMs are made and started.

In a project, up works from its .limabean.yaml (or .limabean.yml), here or
in a parent directory. It creates the VM the file describes, clones the
repository into it and runs its setup. If the VM exists, up starts it and
applies whatever in the file changed: new packs or environment rebuild it
(keeping ~/code), a new size resizes it, new secrets and Claude plugins are
added, and a tighter network is applied. A file can't widen a VM's
network: up prints the command for you to run instead. Setup runs once;
--setup runs it again. Start runs in the background after every up
(unless it's still running), until the VM stops; limabean logs --start
shows its output. --preview shows what up would create or change, and
changes nothing.

With a name, up starts that VM, or creates it if there's none: in a
terminal it asks which packs and secrets it gets, and --with, --allow,
--locked, --ttl and the size flags choose on the command line. Inside a
project, the project's own name (or no name) means its file; any other
name is a VM of its own. Without a terminal, name the VM.

limabean down stops a VM; limabean rm deletes it.

  # .limabean.yaml
  name: myapp                 # default: the directory's name
  env: scratch                # environment template
  packs: [go, db, claude]     # besides core, Docker and Python (limabean packs)
  tools:                      # the project's own mise tools, pinned
    terraform: 1.9.8
  apt: [libpq-dev]            # and Ubuntu packages
  cpus: 6
  memory: 12                  # GiB
  disk: 30                    # GiB of system disk
  network:                    # see: limabean network --help
    allow: [github, npm]
    locked: true
  vars:                       # plain variables for shells, setup and start
    NODE_ENV: development
  secrets: [OPENAI_API_KEY]   # names only; values stay on your Mac
  ports: [3000]               # shown as http://localhost:<port> after up
  claude:                     # plugins for Claude Code in the VM
    marketplaces:
      - marketplace: rossedman/guardrails     # GitHub owner/repo, or an https git URL
        plugins: [guardrails@guardrails]      # plugin@marketplace
  setup: |                    # run once in the clone, with bash -e
    make deps
  start: |                    # run in the clone, in the background, after every up
    docker compose up -d
    npm run dev

The repository cloned is the checkout's origin (or repo: in the file).
Secrets are names only: each must already be set up on your Mac (limabean
secrets ls), and shared secrets are included as always.

Claude plugins go into the VM's ~/.claude/settings.json, and Claude Code
fetches them in the VM when a session starts, so a VM with a network
allowlist needs github (or the marketplace's host).

The file comes with the repository, so up shows what it asks for, or what
it would change, and waits for you to confirm; --yes skips that, for files
you trust. Start commands that changed since you last approved them count
as a change, so a git pull can't slip new ones in.

```
limabean up [name] [environment] [flags]
```

### Examples

```
  # The project you're in: create it, or start it and apply the file's changes
  cd ~/src/myapp && limabean up

  # See what up would change after editing the file, without changing it
  limabean up --preview

  # Run the project's setup again
  limabean up --setup

  # A VM of its own: start dev, or create it (asking which packs and secrets)
  limabean up dev

  # Create one from a script: Go and Node, 4 CPUs, gone in a day
  limabean up scratch --with go,node --cpus 4 --ttl 24h

  # An agent sandbox: GitHub and PyPI only, and no way to undo that
  limabean up agent --with python,claude --allow github --allow python --locked
```

### Options

```
      --allow stringArray    only let the VM reach this domain or preset (repeatable; see: limabean network --help)
      --cpus int             CPUs for the VM (default: the template's, 2)
      --disk string          system disk in GiB (default: the template's, 10); ~/code has its own
  -f, --file string          the project file (default: .limabean.yaml here or in a parent directory)
  -h, --help                 help for up
      --learn                record the domains the VM looks up, to build its allowlist (see: limabean network learn --help)
      --locked               take sudo and Docker away from the VM's user, so its allowlist holds
      --memory string        memory in GiB (default: the template's, 4)
      --preview              show what up would create or change, and change nothing
      --secret stringArray   give this VM a secret: NAME=reference (repeatable)
      --setup                run the project file's setup again in its existing VM
      --ttl duration         delete the VM, whatever is in it, once it's this old, e.g. 2h (needs: limabean autostop on)
      --with strings         give the VM these packs, e.g. go,node (see: limabean packs)
      --without strings      leave out packs the PACKS setting would add
  -y, --yes                  create the project's VM, or apply changes to it, without asking
```

### Options inherited from parent commands

```
      --config string      config file (default ~/.config/limabean/config)
      --no-color           print without color (also NO_COLOR=1)
      --plain              print plain text and never prompt (also LIMABEAN_PLAIN=1)
  -q, --quiet              print only errors (and the data a command returns)
      --templates string   read templates from this directory instead of the built-in copy (also LIMABEAN_TEMPLATES)
  -v, --verbose            also print the limactl and ssh commands limabean runs; wins over --quiet
```

### SEE ALSO

* [limabean](limabean.md)	 - Disposable Linux dev VMs on macOS, ready for SSH and Claude

