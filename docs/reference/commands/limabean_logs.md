## limabean logs

Show a VM's provisioning log, or its project's setup and start output

### Synopsis

Show what happened in a VM. Without flags, logs shows the end of its
provisioning log (cloud-init's output: packages, packs, every boot's
scripts), the place to look when creating a VM fails or hangs.

  --setup   the output of the project's setup, from its last run
  --start   the output of the project's start commands, still running or not
  --lima    Lima's own log on your Mac, for a VM that won't boot

-n sets how many lines (default 100), and -f keeps following new ones
until you press Ctrl-C. Without a name, logs uses the project's VM (from
.limabean.yaml here or in a parent directory), or asks which VM in a
terminal.

A locked VM's user has no sudo, so its provisioning log can't be read from
inside it; --lima still works.

```
limabean logs [name] [flags]
```

### Examples

```
  # Why did creating the VM fail?
  limabean logs dev

  # Follow the project's dev server, started by start: in .limabean.yaml
  limabean logs --start -f

  # The last setup run, all of it
  limabean logs --setup -n 1000
```

### Options

```
      --file string   the project file (default: .limabean.yaml here or in a parent directory)
  -f, --follow        keep printing new lines until Ctrl-C
  -h, --help          help for logs
      --lima          Lima's log on your Mac
  -n, --lines int     how many lines from the end (default 100)
      --setup         the project's setup output
      --start         the project's start output
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

