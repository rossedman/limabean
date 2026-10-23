## limabean get

Clone a repository into a VM with ghq

### Synopsis

Clone a repository into a VM's ~/code with ghq, which lays it out as
~/code/<host>/<owner>/<repo>. A stopped VM is started first.

With only a repository, you pick the VM from a list. Flags after the
repository are passed to "ghq get": -p clones over SSH, which private
repositories need unless a GitHub token is installed (limabean github login).

```
limabean get [name] <repo> [ghq flags...] [flags]
```

### Examples

```
  # Into dev: ~/code/github.com/rossedman/limabean
  limabean get dev rossedman/limabean

  # A full URL works too
  limabean get dev https://github.com/charmbracelet/gum

  # A private repository over SSH
  limabean get dev -p acme/private-repo

  # Pick the VM from a list
  limabean get acme/private-repo -p
```

### Options

```
  -h, --help   help for get
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

