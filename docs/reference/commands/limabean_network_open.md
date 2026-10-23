## limabean network open

Let a VM reach any host again

### Synopsis

Remove a VM's allowlist, so it can reach any host, and stop recording its
lookups if it was learning. A locked VM stays
locked: use "limabean network unlock" too to give its user sudo back.

```
limabean network open <name> [flags]
```

### Examples

```
  limabean network open agent
```

### Options

```
  -h, --help   help for open
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

