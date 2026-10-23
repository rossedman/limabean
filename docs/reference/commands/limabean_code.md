## limabean code

Open a VM, or a repository in it, in VS Code, Cursor or another editor

### Synopsis

Open a VM in your editor over SSH, using its Remote - SSH extension. A stopped VM is started first.

With a repository, the editor opens that clone under ~/code. It's matched
against the VM's ghq clones the way "ghq list" matches: limabean,
rossedman/limabean and github.com/rossedman/limabean all find the same one.
If several match, you pick one. An absolute path in the VM works too.
Without one, the editor opens ~/code.

With a single argument that isn't a VM's name, it's taken as a repository
and you pick the VM.

The editor is the IDE setting, or --ide: code, code-insiders, cursor,
windsurf, codium, or any other editor whose command line takes VS Code's
--remote flag. Without one, or when the one named isn't installed,
limabean opens the first of those it finds. It finds an editor on your
PATH or inside its app in /Applications, so "Install 'code' command in
PATH" isn't needed. It connects to lima-<name> through ~/.ssh/config, so
that needs Lima's Include (limabean doctor checks it).

VS Code, VS Code Insiders and VSCodium need a Remote - SSH extension to
open a VM, and each keeps its own extensions. If it's missing, limabean
offers to install it: ms-vscode-remote.remote-ssh, or
jeanp413.open-remote-ssh for VSCodium. Cursor and Windsurf ship their own,
so limabean doesn't check them.

```
limabean code [name] [repo] [flags]
```

### Examples

```
  # dev's ~/code in VS Code
  limabean code dev

  # One repository
  limabean code dev limabean
  limabean code dev rossedman/limabean

  # In Cursor, or VS Code Insiders, this time
  limabean code dev limabean --ide cursor
  limabean code dev --ide code-insiders

  # Always use VS Code Insiders
  echo 'IDE="code-insiders"' >> ~/.config/limabean/config
```

### Options

```
  -h, --help         help for code
      --ide string   editor to open, e.g. code-insiders or cursor (default: the IDE setting, or the first one installed)
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

