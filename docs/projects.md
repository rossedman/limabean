# Projects: `.limabean.yaml`

A repository can say what VM it needs in a `.limabean.yaml` file at its root.
Then anyone with limabean gets the right VM with one command, from the
checkout on their Mac:

```bash
cd ~/src/myapp
limabean up
```

`up` creates the VM the file describes, clones the repository into it and
runs its setup. If the VM already exists, `up` starts it.

## The file

Every key is optional:

```yaml
# .limabean.yaml
name: myapp                 # the VM's name; default: the directory's name
env: scratch                # environment template
packs: [rust, claude]       # added to the default packs
without: [kubernetes]       # default packs to leave out
cpus: 6
memory: 12                  # GiB
disk: 30                    # GiB of system disk, for Docker images
network:
  allow: [github, npm]      # only these domains and presets
  locked: true              # and nothing in the VM can change that
secrets: [OPENAI_API_KEY]   # names only; values stay on your Mac
ports: [3000]               # shown as http://localhost:<port> after up
setup: |                    # run once in the clone, with bash -e
  make deps
  make db-migrate
```

- **Packs** are the same as `new --with` and `--without`: see
  [Packs](packs.md).
- **Network** is the same as `new --allow` and `--locked`: see
  [Sandboxes](network.md). To work out what to allow, create the VM once
  with `limabean new <name> --learn`.
- **Secrets** are names only. Each must already be set up on your Mac
  (`limabean secrets ls`), for every VM or for another one; up gives the VM
  those, plus your shared secrets as always, and says which names it
  couldn't find.
- **The repository** cloned is the checkout's `origin` remote. Set `repo:`
  to clone something else.
- **Setup** runs once, when up creates the VM, in the clone, as you. If it
  fails, the VM is kept so you can look; fix the setup and run
  `limabean rm <name> && limabean up` to start over.

Unknown keys are errors, so a typo doesn't go unnoticed.

## Trust

The file comes with the repository, and its setup runs in a VM that gets
your shared secrets. So `up` never acts on it blindly:

- In a terminal, it shows what the file asks for, setup commands included,
  and asks before creating anything.
- Without a terminal it shows the same and stops, unless you pass `--yes`.
- The file can only **name** secrets. It can't say where a value comes from,
  so it can't run a command on your Mac (as a `cmd:` reference would) or
  read a Keychain item you didn't already give VMs.

Read a file from someone else's repository before saying yes, as you would
a `Makefile`.

## Commands

| Command | Does |
|---|---|
| `limabean up` | Creates the VM from `.limabean.yaml` here or in a parent directory, or starts it |
| `limabean up --yes` | The same without asking, for a file you trust |
| `limabean up --file <path>` | Uses another file |
