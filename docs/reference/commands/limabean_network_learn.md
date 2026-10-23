## limabean network learn

Record what a VM reaches, to build its allowlist

### Synopsis

Find out which domains a VM's work needs. While it learns, the VM can reach
any host, and every DNS lookup it or its containers make is recorded: run
the task, the tests or the install the way an agent would.

"limabean network log" shows what was looked up. --done ends learning and
allows the domains seen (a.b.example.com is allowed as example.com), on
top of any already on the allowlist. The VM is limited to them from then
on, so lock it too if an agent will use it.

Lookups are recorded until the VM restarts.

```
limabean network learn <name> [flags]
```

### Examples

```
  limabean network learn agent
  limabean ssh agent make test
  limabean network log agent
  limabean network learn agent --done
  limabean network lock agent
```

### Options

```
      --done   stop learning, and allow the domains that were looked up
  -h, --help   help for learn
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

