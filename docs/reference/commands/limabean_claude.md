## limabean claude

Keep VMs listed as SSH connections in Claude Desktop

### Synopsis

Claude Desktop reads SSH connections from the "sshConfigs" list in
~/.claude/settings.json. limabean adds an entry for every VM it creates
("dev (limabean)", host lima-dev) and removes it when the VM is deleted,
touching only entries whose id starts with "limabean-". Other settings are
kept, and a file that isn't valid JSON is left alone.

On first connect, Claude Desktop installs its own copy of Claude Code in the
VM. Set CLAUDE_DESKTOP="no" in the config to turn this integration off.

### Examples

```
  # Re-register every VM and drop deleted ones
  limabean claude sync
```

### Options

```
  -h, --help   help for claude
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
* [limabean claude sync](limabean_claude_sync.md)	 - Register running VMs in Claude Desktop and remove the rest

