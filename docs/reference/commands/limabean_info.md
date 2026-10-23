## limabean info

Show what a VM knows about itself

### Synopsis

Show what limabean-info prints inside a VM: its packs and tools, its
network policy, the names of its secrets (never their values) and, for a
project's VM, the repository, the clone, the ports and the logs.

The VM must be running. Inside a project's checkout, it uses the
project's VM when you don't name one; otherwise leave out the name to pick
one from a list. VMs made before limabean-info need a rebuild to have it.

```
limabean info [name] [flags]
```

### Examples

```
  limabean info dev

  # As JSON, for scripts
  limabean info dev --json | jq .network
```

### Options

```
  -h, --help   help for info
      --json   print JSON
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

