## limabean packs

List the packs of tools a VM can get

### Synopsis

List the packs of tools VMs can get and what each installs. Every VM has
core, the shell tools limabean itself relies on, and Docker and Python;
everything else is a pack you choose:

  limabean up api --with go,db           # or pick them in up's form
  limabean rebuild dev --with ruby       # add one to an existing VM
  packs: [go, db]                        # in a project's .limabean.yaml

A VM keeps its packs through rebuilds. The PACKS setting adds packs to
every VM made with new or run (it's empty unless you set it; project files
ignore it). Versions are pinned in templates/packs/*.toml, and
"limabean bump" updates them.

```
limabean packs [flags]
```

### Examples

```
  limabean packs

  # Give every new VM Go and Node
  echo 'PACKS="go,node"' >> ~/.config/limabean/config
```

### Options

```
  -h, --help   help for packs
      --json   print a JSON array, for scripts
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

