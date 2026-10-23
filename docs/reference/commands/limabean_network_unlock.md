## limabean network unlock

Give a VM's user sudo and Docker back

### Synopsis

Unlock a VM: its user gets passwordless sudo back at once, and Docker on
the next boot (limabean down, then up). Its allowlist, if it has one, stays,
but anything in the VM can now turn it off.

```
limabean network unlock <name> [flags]
```

### Examples

```
  limabean network unlock agent
```

### Options

```
  -h, --help   help for unlock
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

