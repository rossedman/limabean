## limabean snapshot save

Save a snapshot of a VM

### Synopsis

Save a VM's disks as a snapshot: its system, and ~/code when that's on its
own disk. A running VM is shut down for the few seconds it takes and then
started again.

The snapshot is named after the date and time unless you name it. Saving
under a name that's taken replaces that snapshot.

```
limabean snapshot save [name] [snapshot] [flags]
```

### Examples

```
  limabean snapshot save dev before-refactor

  # Named after the time, e.g. 2026-10-17-1530
  limabean snapshot save dev
```

### Options

```
  -h, --help   help for save
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

