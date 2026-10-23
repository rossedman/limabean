## limabean ps

Show the project's VM: status, start, ports and pending changes

### Synopsis

Show the state of the project you're in, from its .limabean.yaml (or
.limabean.yml) here or in a parent directory:

  - its VM: running, stopped, or not created yet
  - its start commands: running, stopped, or changed and not approved yet
  - the ports programs in it listen on, and whether each reaches your Mac,
    plus any port the file lists that nothing listens on
  - what "limabean up" would change to match the file, and the network
    changes only you can make

ps changes nothing. For every VM at once, use limabean ls.

```
limabean ps [flags]
```

### Examples

```
  cd ~/src/myapp && limabean ps

  # For scripts
  limabean ps --json
```

### Options

```
  -f, --file string   the project file (default: .limabean.yaml here or in a parent directory)
  -h, --help          help for ps
      --json          print it as JSON, for scripts
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

