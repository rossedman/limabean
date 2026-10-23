## limabean images prune

Delete images that are no longer used

### Synopsis

Delete images built from templates that have changed since (they're never
used again), images nobody has copied for 30 days, and image VMs limabean
has no record of. "limabean autostop run" does this too.

```
limabean images prune [flags]
```

### Examples

```
  limabean images prune
```

### Options

```
  -h, --help   help for prune
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

* [limabean images](limabean_images.md)	 - Manage the images that make new VMs start in seconds

