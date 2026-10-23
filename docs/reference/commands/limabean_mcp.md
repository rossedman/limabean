## limabean mcp

Let Claude manage your VMs, as an MCP server

### Synopsis

Run an MCP server on stdin and stdout, so Claude Code or Claude Desktop on
your Mac can manage limabean VMs: list, create, start and stop them, run
commands in them, clone repositories, see forwarded ports and save
snapshots. An agent can then make itself a scratch VM, work in it, and
checkpoint it before something risky. For a checkout with a
.limabean.yaml, the project tools plan, bring up, inspect and stop its VM
as "limabean up" would; Claude Code asks you before every project_up.

What the tools can't do:
  - run anything on your Mac: commands run inside VMs only, and the
    project tools read only a checkout's .limabean.yaml and .git/config
  - read secret values, or add secrets (a cmd: secret would run on the Mac)
  - widen a VM's network, from a project file or otherwise
  - delete, rebuild or restore a VM, unless you pass --allow-destructive.
    Even then, deleting stops at unsaved work as "limabean rm" does, and
    Claude Code asks you before every call to them.

Commands in a VM run as you, with the VM's secrets in their environment,
and can reach anything the VM can, including GitHub with its token.

```
limabean mcp [flags]
```

### Examples

```
  # Claude Code
  claude mcp add limabean -- limabean mcp

  # Claude Desktop: in claude_desktop_config.json
  #   "mcpServers": { "limabean": { "command": "/opt/homebrew/bin/limabean", "args": ["mcp"] } }

  # Also let Claude delete, rebuild and restore VMs
  claude mcp add limabean -- limabean mcp --allow-destructive
```

### Options

```
      --allow-destructive   also offer tools that delete, rebuild or restore VMs
  -h, --help                help for mcp
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

