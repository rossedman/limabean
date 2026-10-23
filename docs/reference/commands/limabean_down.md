## limabean down

Stop a VM, or the project's

### Synopsis

Stop a VM. Everything in it is kept, ~/code included, and "limabean up"
starts it again (in a project, applying any changes to its file). To
delete a VM, use limabean rm.

Without a name, down stops the project's VM: the one .limabean.yaml (or
.limabean.yml) here or in a parent directory describes. Outside a project
it asks which VM in a terminal.

Its Claude Desktop connection is removed while it's stopped (Claude Desktop
shows an error for a VM it can't reach) and added back when it starts.

With --idle, only VMs nobody has used for that long are stopped, and
without a name every running VM is checked. A VM is in use while:
  - a terminal in it (a shell, tmux, limabean ssh) had input or output
  - Claude Desktop or an editor is connected to it
  - Claude Code is running in it, e.g. a headless claude -p
  - its 5-minute load average is 1 or more, e.g. a build or busy containers
  - it started less than that long ago
"limabean autostop on" runs this every few minutes.

```
limabean down [name] [flags]
```

### Examples

```
  # The project you're in
  cd ~/src/myapp && limabean down

  # A VM by name
  limabean down dev

  # Every VM unused for two hours, and a dry run of the same
  limabean down --idle 2h
  limabean down --idle 2h --preview
```

### Options

```
  -f, --file string     the project file (default: .limabean.yaml here or in a parent directory)
  -h, --help            help for down
      --idle duration   only stop VMs unused for this long, e.g. 2h; without a name, every running VM is checked
      --preview         with --idle, show which VMs would stop without stopping them
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

