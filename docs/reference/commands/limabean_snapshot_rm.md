## limabean snapshot rm

Delete a snapshot

### Synopsis

Delete one of a VM's snapshots, freeing the space it kept. The VM itself is
left alone.

```
limabean snapshot rm <name> <snapshot> [flags]
```

### Examples

```
  limabean snapshot rm dev before-refactor
```

### Options

```
  -h, --help   help for rm
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

