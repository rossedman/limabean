## limabean doctor

Check everything VMs depend on

### Synopsis

Check your Mac for everything limabean and its VMs rely on, and print the
exact fix for anything missing:

  Lima          limactl 2.0 or newer
  SSH config    Lima's host entries are included at the top of ~/.ssh/config
  SSH agent     a key is loaded, so VMs can push with it
  Git config    the directory mounted into VMs has a git config
  Claude        Desktop's settings file can be updated
  GitHub        the saved token works, and when it expires
  Secrets       every reference can be looked up (values aren't read)
  AWS           whether ~/.aws will be mounted
  Project       in a checkout with a .limabean.yaml: the file loads, and its
                packs, network presets and secrets exist on this Mac; and
                whether its VM matches it

It exits non-zero if something needs fixing, so it can gate scripts.

```
limabean doctor [flags]
```

### Examples

```
  limabean doctor

  # In a script
  limabean doctor --plain >/dev/null || echo "fix limabean first"
```

### Options

```
  -h, --help   help for doctor
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

