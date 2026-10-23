## limabean autostop on

Stop VMs that nobody has used for a while

### Synopsis

Install a launchd agent that runs "limabean autostop run --idle <after>"
every 10 minutes while you're logged in: it stops VMs nobody has used for
that long, and deletes VMs made with "new --ttl" once they expire. Running
it again changes the limit.

The agent runs the limabean on your PATH (so upgrades are picked up) and
finds limactl where it is now. Its output goes to
~/Library/Logs/limabean-autostop.log.

```
limabean autostop on [flags]
```

### Examples

```
  limabean autostop on
  limabean autostop on --after 45m
```

### Options

```
      --after duration   stop a VM once nobody has used it for this long (default 2h0m0s)
  -h, --help             help for on
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

* [limabean autostop](limabean_autostop.md)	 - Stop VMs automatically when nobody is using them

