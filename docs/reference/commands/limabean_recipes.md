## limabean recipes

List starter .limabean.yaml files, or write one into a repository

### Synopsis

Recipes are starter project files for common kinds of VMs: a Go service, a
web frontend, a Rails app, a Kubernetes workspace, a sandbox for Claude Code
and more. Each is a .limabean.yaml with comments, ready to edit.

Without a name, recipes lists them. With one, it writes that recipe to
.limabean.yaml in the current directory, so "limabean up" can use it; it
won't replace a project file that's already there unless you pass --overwrite.
--print writes it to standard output instead.

See what a recipe asks for before you use it: the file's keys are described
in "limabean up --help". The file's first line points editors at its JSON
Schema, so VS Code, Cursor and others complete and check its keys (with the
YAML language server, e.g. the Red Hat YAML extension).

```
limabean recipes [name] [flags]
```

### Examples

```
  limabean recipes

  # Start a Go service's project file, then bring the VM up
  cd ~/src/api && limabean recipes go-service && limabean up

  # Read one first
  limabean recipes claude-agent --print
```

### Options

```
  -h, --help        help for recipes
      --overwrite   replace a project file that's already here
      --print       write the recipe to standard output
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

