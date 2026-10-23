## limabean ssh

Open a shell in a VM, or run a command in it

### Synopsis

Open an interactive shell in a VM, or run a single command in it. A stopped
VM is started first.

Leave out the name to pick a VM from a list. Everything after the name is
the command to run; flags after the name belong to that command, not to
limabean.

Inside a project's checkout, the project's VM is the default, and the shell
or command runs in its clone of the repository rather than the home
directory. Start with -- to give a command without naming the VM.

This is the same as "ssh lima-<name>", but works even if your ~/.ssh/config
doesn't include Lima's host entries.

```
limabean ssh [name] [command...] [flags]
```

### Examples

```
  # A shell in dev
  limabean ssh dev

  # Pick the VM from a list
  limabean ssh

  # Run one command
  limabean ssh dev docker ps -a
  limabean ssh dev -- ls -la ~/code

  # In a project's checkout: its tests, in the VM's clone
  limabean ssh -- make test
```

### Options

```
  -h, --help   help for ssh
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

