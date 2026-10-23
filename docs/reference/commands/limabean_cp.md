## limabean cp

Copy files between your Mac and a VM

### Synopsis

Copy files between your Mac and a VM, or between VMs, with limactl copy.
Write a VM path as <vm>:<path>; a relative one is in the VM user's home
directory. A stopped VM is started first.

Copy directories with -r.

```
limabean cp <source>... <target> [flags]
```

### Examples

```
  # A file into the VM's home directory
  limabean cp ./notes.md dev:

  # A directory out of the VM
  limabean cp -r dev:code/github.com/me/app/dist ./dist

  # Between two VMs
  limabean cp dev:report.html agent:
```

### Options

```
  -h, --help        help for cp
  -r, --recursive   copy directories
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

