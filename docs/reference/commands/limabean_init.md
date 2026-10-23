## limabean init

Set up limabean, or write a repository's .limabean.yaml

### Synopsis

Walk through first-time setup, then run "limabean doctor":

  1. Where your git config lives on the Mac. A ~/.gitconfig symlinked into a
     dotfiles repository is followed, and that directory is suggested.
  2. Where repositories go inside VMs (default ~/code).
  3. Whether to add "Include ~/.lima/*/ssh.config" to the top of
     ~/.ssh/config (a backup is kept), so ssh lima-<name> and Claude Desktop
     can reach VMs.
  4. Optionally, a GitHub token for gh and https pushes in VMs.

Settings are saved to ~/.config/limabean/config. Without a terminal, the
suggested values are saved and the SSH config isn't changed.

With --project, init writes a .limabean.yaml for the repository you're in
instead, from what's in it: go.mod, package.json and its lockfile,
pyproject.toml, Cargo.toml, Gemfile, mix.exs, pom.xml, a compose file,
.tool-versions and the like, here and in the directories directly below. It
picks the packs, the setup commands, likely ports, and tools no pack has,
and lists credentials named in .env.example as secrets to set up. It only
reads files; nothing in the repository runs. In a terminal it shows what it
found and asks before writing. --print writes the file to standard output
instead, --overwrite replaces a project file that's already there, and
--dir looks at another directory than the current one.

```
limabean init [flags]
```

### Examples

```
  limabean init

  # Accept every suggestion, e.g. in a provisioning script
  limabean init --plain

  # A project file for this repository, then its VM
  cd ~/src/myapp && limabean init --project && limabean up

  # See what it would write, for another checkout
  limabean init --project --print --dir ~/src/api
```

### Options

```
      --dir string   with --project, the repository to look at, instead of the current directory
  -h, --help         help for init
      --overwrite    with --project, replace a project file that's already here
      --print        with --project, write it to standard output
      --project      write a .limabean.yaml for the repository here instead
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

