## limabean snapshot ls

List a VM's snapshots

### Synopsis

List a VM's snapshots, oldest first, with when each was saved and whether
it includes ~/code (VMs without a code disk only have their system saved).

```
limabean snapshot ls [name] [flags]
```

### Examples

```
  limabean snapshot ls dev

  # For scripts
  limabean snapshot ls dev --json | jq -r '.[-1].tag'
```

### Options

```
  -h, --help   help for ls
      --json   print a JSON array, for scripts
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

