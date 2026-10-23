## limabean secrets ls

List secrets and where their values come from

### Synopsis

List every secret with where its value comes from and which VMs get it.
Values are never shown, and plain values are only marked as such.

With --vm, list exactly what that VM gets: the shared secrets merged with
its own, its own winning when both set the same name.

```
limabean secrets ls [flags]
```

### Examples

```
  limabean secrets ls

  # What dev gets
  limabean secrets ls --vm dev
```

### Options

```
  -h, --help        help for ls
      --vm string   show what this VM gets
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

* [limabean secrets](limabean_secrets.md)	 - Manage the environment variables and secrets VMs get

