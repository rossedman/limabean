## limabean network ls

Show a VM's network policy

### Synopsis

Show whether a VM's network is open or limited to an allowlist, the
domains it may reach, and whether it's locked. ntp.ubuntu.com is always
allowed alongside an allowlist, for the clock.

```
limabean network ls [name] [flags]
```

### Examples

```
  limabean network ls agent
```

### Options

```
  -h, --help   help for ls
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

