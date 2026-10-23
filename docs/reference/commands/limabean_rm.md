## limabean rm

Delete a VM

### Synopsis

Delete a VM and everything limabean keeps for it: the disk its ~/code is
on, its Claude Desktop entry, its saved environment, its own secrets file
(secrets.<name>.env) and the Keychain items limabean created for those
secrets. Shared secrets are kept.

Before deleting, rm checks every git repository under ~/code in the VM. If
any has uncommitted changes, stashes or commits that aren't on a remote, it
stops and lists them. In a terminal it then asks you to confirm. --yes
skips the question; --force skips both.

A VM already deleted with "limactl delete" can still be named: rm cleans up
what's left of it.

--expired deletes every VM made with "new --ttl" whose time is up, without
checks; the autostop agent does this every 10 minutes.

```
limabean rm [name] [flags]
```

### Examples

```
  limabean rm dev

  # Pick the VM from a list
  limabean rm

  # Don't ask, but still check for unsaved work
  limabean rm scratch --yes

  # No checks, no questions
  limabean rm scratch --force

  # Every VM past its --ttl
  limabean rm --expired
```

### Options

```
      --expired   delete every VM whose --ttl is up
  -f, --force     skip the unsaved-work check and the confirmation
  -h, --help      help for rm
  -y, --yes       skip the confirmation; the unsaved-work check still runs
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

