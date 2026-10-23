## limabean network lock

Take sudo and Docker away from a VM's user

### Synopsis

Lock a VM: its user loses passwordless sudo, the sudo and docker groups,
and the Docker socket, so nothing running as that user (an agent
included) can become root and turn its allowlist off. The lock is
re-applied on every boot.

Lock before handing a VM to an agent: a root process started before the
lock keeps running. "limabean network unlock" undoes it.

```
limabean network lock <name> [flags]
```

### Examples

```
  limabean network lock agent
```

### Options

```
  -h, --help   help for lock
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

