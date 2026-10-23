## limabean secrets sync

Install secrets in running VMs

### Synopsis

Look up every secret on the Mac and write the values into VMs, replacing
what they had: secrets that were removed disappear too.

Without a name, every running VM is updated and stopped ones are listed
(they keep their last secrets until synced). With a name, that VM is
updated, and started first if it's stopped.

A secret that can't be looked up (a missing Keychain item, a locked
1Password, an unset variable) is reported and left out; the rest are still
installed. Run it after rotating a token or editing a secrets file.

```
limabean secrets sync [name] [flags]
```

### Examples

```
  limabean secrets sync

  # Just dev, starting it if needed
  limabean secrets sync dev
```

### Options

```
  -h, --help   help for sync
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

* [limabean secrets](limabean_secrets.md)	 - Manage the environment variables and secrets VMs get

