## limabean network log

Show the names a VM looked up, and which were blocked

### Synopsis

Show every name a running VM, or a container in it, looked up since it
started, how often, and whether its allowlist let it through. When an
install or a clone fails in a sandbox, the domain it needed is here.

Lookups are recorded while the VM has an allowlist or is learning
(limabean network learn); an open VM's aren't.

```
limabean network log <name> [flags]
```

### Examples

```
  limabean network log agent

  # Only what the allowlist stopped
  limabean network log agent --blocked
```

### Options

```
      --blocked   only show names the allowlist blocked
  -h, --help      help for log
      --json      print a JSON array, for scripts
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

* [limabean network](limabean_network.md)	 - Limit which hosts a VM can reach, and lock it down

