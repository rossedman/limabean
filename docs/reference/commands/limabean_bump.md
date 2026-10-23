## limabean bump

Update the pinned tool versions in mise TOML files

### Synopsis

Look up the newest version of every tool pinned in mise TOML files and
rewrite the pins, keeping comments and layout. Without files it updates the
packs, templates/packs/*.toml, and any templates/mise/*.toml, under the
current directory: a limabean checkout, or your own templates folder.

Each pin stays within its major version (its minor below 1.0, where minors
can break, and its vendor, like temurin- for Java) unless you pass --major.
Newer majors are listed either way.

Versions come from "mise latest", run on your Mac if mise is installed
(brew install mise) or in a running VM with --vm, since every VM has mise.
VMs pick up new pins when they're rebuilt.

```
limabean bump [file...] [flags]
```

### Examples

```
  # In a limabean checkout
  limabean bump

  # See what would change
  limabean bump --preview

  # Without mise on the Mac
  limabean bump --vm dev

  # One pack, majors included
  limabean bump templates/packs/node.toml --major
```

### Options

```
  -h, --help        help for bump
      --major       also move pins to new major versions
      --preview     show what would change without writing
      --vm string   look versions up with mise in this running VM
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

