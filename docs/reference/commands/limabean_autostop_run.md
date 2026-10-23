## limabean autostop run

Stop idle VMs and delete expired ones, once

### Synopsis

Do what the autostop agent does every 10 minutes, once: stop every VM
nobody has used for --idle (as "limabean down --idle"), and delete every VM
made with "new --ttl" that has expired (as "limabean rm --expired"), and
delete images that are no longer used (as "limabean images prune").

```
limabean autostop run [flags]
```

### Examples

```
  limabean autostop run --idle 2h
```

### Options

```
  -h, --help            help for run
      --idle duration   stop VMs unused for this long (default 2h0m0s)
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

