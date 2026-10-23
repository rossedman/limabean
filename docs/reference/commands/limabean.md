## limabean

Disposable Linux dev VMs on macOS, ready for SSH and Claude

### Synopsis

limabean creates Ubuntu dev VMs on your Mac from layered Lima templates.

Every VM gets a version-pinned toolchain (Docker, Go, Python, Kubernetes and
Terraform tooling, and more), your git identity, read-only access to your AWS
profiles, and the secrets you choose. It's reachable as ssh lima-<name>, and
is registered as an SSH connection in Claude Desktop. Your SSH keys stay on
the Mac: git pushes through ssh-agent forwarding, or over https with a
GitHub token kept in your Keychain.

limabean drives limactl, so plain Lima commands keep working alongside it.
In a terminal it shows spinners, pickers and forms; when output isn't a
terminal, or with --plain, it prints plain text and never prompts.

### Examples

```
  # Set up once: settings, SSH config, a GitHub token
  limabean init

  # Create a VM and connect to it
  limabean up dev
  limabean ssh dev

  # Give every VM an API key from 1Password
  limabean secrets set OPENAI_API_KEY op://Private/OpenAI/credential

  # See what you have
  limabean ls
```

### Options

```
      --config string      config file (default ~/.config/limabean/config)
  -h, --help               help for limabean
      --no-color           print without color (also NO_COLOR=1)
      --plain              print plain text and never prompt (also LIMABEAN_PLAIN=1)
  -q, --quiet              print only errors (and the data a command returns)
      --templates string   read templates from this directory instead of the built-in copy (also LIMABEAN_TEMPLATES)
  -v, --verbose            also print the limactl and ssh commands limabean runs; wins over --quiet
```

### SEE ALSO

* [limabean autostop](limabean_autostop.md)	 - Stop VMs automatically when nobody is using them
* [limabean bump](limabean_bump.md)	 - Update the pinned tool versions in mise TOML files
* [limabean claude](limabean_claude.md)	 - Keep VMs listed as SSH connections in Claude Desktop
* [limabean clone](limabean_clone.md)	 - Copy a VM, in seconds
* [limabean code](limabean_code.md)	 - Open a VM, or a repository in it, in VS Code, Cursor or another editor
* [limabean completion](limabean_completion.md)	 - Print a shell completion script
* [limabean config](limabean_config.md)	 - Show your settings and where each comes from
* [limabean cp](limabean_cp.md)	 - Copy files between your Mac and a VM
* [limabean doctor](limabean_doctor.md)	 - Check everything VMs depend on
* [limabean down](limabean_down.md)	 - Stop a VM, or the project's
* [limabean get](limabean_get.md)	 - Clone a repository into a VM with ghq
* [limabean github](limabean_github.md)	 - Give VMs a GitHub token for gh, the API and https pushes
* [limabean images](limabean_images.md)	 - Manage the images that make new VMs start in seconds
* [limabean info](limabean_info.md)	 - Show what a VM knows about itself
* [limabean init](limabean_init.md)	 - Set up limabean, or write a repository's .limabean.yaml
* [limabean logs](limabean_logs.md)	 - Show a VM's provisioning log, or its project's setup and start output
* [limabean ls](limabean_ls.md)	 - List your VMs
* [limabean mcp](limabean_mcp.md)	 - Let Claude manage your VMs, as an MCP server
* [limabean network](limabean_network.md)	 - Limit which hosts a VM can reach, and lock it down
* [limabean open](limabean_open.md)	 - Open a port a VM forwards in your browser
* [limabean packs](limabean_packs.md)	 - List the packs of tools a VM can get
* [limabean ports](limabean_ports.md)	 - List the ports a VM forwards to your Mac
* [limabean ps](limabean_ps.md)	 - Show the project's VM: status, start, ports and pending changes
* [limabean rebuild](limabean_rebuild.md)	 - Recreate a VM from the current templates
* [limabean recipes](limabean_recipes.md)	 - List starter .limabean.yaml files, or write one into a repository
* [limabean remote](limabean_remote.md)	 - Add a VM's clone of this repository as a git remote on your Mac
* [limabean rm](limabean_rm.md)	 - Delete a VM
* [limabean run](limabean_run.md)	 - Run a command in a new VM, and delete it after
* [limabean secrets](limabean_secrets.md)	 - Manage the environment variables and secrets VMs get
* [limabean snapshot](limabean_snapshot.md)	 - Save a VM's state and roll back to it later
* [limabean ssh](limabean_ssh.md)	 - Open a shell in a VM, or run a command in it
* [limabean up](limabean_up.md)	 - Create or start a VM, or bring a project's VM up to date
* [limabean version](limabean_version.md)	 - Print limabean's version

