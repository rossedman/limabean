## limabean images ls

List images

### Synopsis

List the images new VMs are copied from: the environment and packs each is
for, and how old it is. A stale image is over a week old and is replaced
the next time it's used; an outdated one was built from templates that
have changed since, and is no longer used ("limabean images prune"
deletes it).

```
limabean images ls [flags]
```

### Examples

```
  limabean images ls
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

* [limabean images](limabean_images.md)	 - Manage the images that make new VMs start in seconds

