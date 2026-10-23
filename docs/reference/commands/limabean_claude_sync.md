## limabean claude sync

Register running VMs in Claude Desktop and remove the rest

### Synopsis

Make Claude Desktop's SSH connections match your running VMs: add an entry
for each running VM built from limabean's templates, and remove entries for
VMs that are stopped or no longer exist (Claude Desktop shows an error for a
connection it can't reach). "new", "ssh", "get", "stop", "rm" and "ls"
already do this as they go; sync is for VMs started or stopped with limactl,
or after editing settings.json by hand.

Restart Claude Desktop if a VM doesn't show up in its environment dropdown.

```
limabean claude sync [flags]
```

### Examples

```
  limabean claude sync
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

* [limabean claude](limabean_claude.md)	 - Keep VMs listed as SSH connections in Claude Desktop

