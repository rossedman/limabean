## limabean autostop

Stop VMs automatically when nobody is using them

### Synopsis

Stop idle VMs in the background, so forgotten ones don't hold on to memory
and battery, and delete VMs made with "new --ttl" once they expire.
"autostop on" installs a launchd agent in your login session that runs
"limabean autostop run" every 10 minutes; see "limabean down --help" for
what counts as in use.

A stopped VM keeps its disk. "limabean ssh", "code" and "start" bring it
back, and Claude Desktop lists it again once it runs.

### Examples

```
  # Stop VMs nobody has used for two hours
  limabean autostop on

  # A longer grace period
  limabean autostop on --after 4h

  limabean autostop status
  limabean autostop off
```

### Options

```
  -h, --help   help for autostop
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

* [limabean](limabean.md)	 - Disposable Linux dev VMs on macOS, ready for SSH and Claude
* [limabean autostop off](limabean_autostop_off.md)	 - Stop stopping idle VMs
* [limabean autostop on](limabean_autostop_on.md)	 - Stop VMs that nobody has used for a while
* [limabean autostop run](limabean_autostop_run.md)	 - Stop idle VMs and delete expired ones, once
* [limabean autostop status](limabean_autostop_status.md)	 - Show whether idle VMs are stopped automatically

