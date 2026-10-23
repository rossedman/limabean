## limabean rebuild

Recreate a VM from the current templates

### Synopsis

Delete a VM and create it again, with the same name and environment, from
the templates as they are now. This is how changes to templates, tools,
shell config or limabean itself reach an existing VM.

The VM's disk is erased. ~/code is kept when it's on its own disk, which
VMs created with the CODE_DISK setting on (the default) have; otherwise
repositories are lost and need cloning again with "limabean get". Its
secrets, Claude Desktop entry and settings are kept and reinstalled.

Without a code disk, rebuild first checks for uncommitted, stashed or
unpushed work, like rm. In a terminal it asks you to confirm. --yes skips
the question; --force skips both.

```
limabean rebuild [name] [flags]
```

### Examples

```
  limabean rebuild dev

  # Pick the VM from a list
  limabean rebuild

  # Add Rust, drop Kubernetes
  limabean rebuild dev --with rust --without kubernetes

  # No checks, no questions
  limabean rebuild dev --force
```

### Options

```
  -f, --force             skip the unsaved-work check and the confirmation
  -h, --help              help for rebuild
      --with strings      add these packs, e.g. rust
      --without strings   remove these packs
  -y, --yes               skip the confirmation; the unsaved-work check still runs
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

