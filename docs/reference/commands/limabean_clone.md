## limabean clone

Copy a VM, in seconds

### Synopsis

Make a new VM that starts as an exact copy of another: its system, its
packs, its ~/code with every clone and uncommitted change, its secrets and
its network policy. The disks are APFS clones, so it takes seconds and only
uses space for what changes afterwards.

The VM being copied is shut down for those seconds and started again if it
was running. Its snapshots aren't copied.

Use it to try something risky on a copy, or to give several agents the same
starting point and keep the one that worked.

```
limabean clone <name> <new-name> [flags]
```

### Examples

```
  limabean clone dev dev-experiment

  # Three identical sandboxes
  for i in 1 2 3; do limabean clone agent agent-$i; done
```

### Options

```
  -h, --help   help for clone
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

