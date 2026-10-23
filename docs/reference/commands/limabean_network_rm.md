## limabean network rm

Stop a VM reaching some domains

### Synopsis

Remove domains or presets from a VM's allowlist. Removing the last one
doesn't open the network: use "limabean network open" for that.

```
limabean network rm <name> <domain|preset>... [flags]
```

### Examples

```
  limabean network rm agent python
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

* [limabean network](limabean_network.md)	 - Limit which hosts a VM can reach, and lock it down

