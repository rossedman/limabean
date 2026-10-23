## limabean snapshot restore

Put a VM back the way a snapshot saved it

### Synopsis

Restore a snapshot: the VM's system, and ~/code if the snapshot includes it,
go back to how they were when it was saved. Everything changed since then
is lost, pushed or not. A running VM is shut down and started again.

Leave out the snapshot to restore the latest one. In a terminal it asks
you to confirm; --yes doesn't ask.

```
limabean snapshot restore [name] [snapshot] [flags]
```

### Examples

```
  limabean snapshot restore dev before-refactor

  # The latest snapshot, without asking
  limabean snapshot restore dev --yes
```

### Options

```
  -h, --help   help for restore
  -y, --yes    don't ask for confirmation
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

* [limabean snapshot](limabean_snapshot.md)	 - Save a VM's state and roll back to it later

