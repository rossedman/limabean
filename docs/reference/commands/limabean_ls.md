## limabean ls

List your VMs

### Synopsis

List the VMs built from limabean's templates: their status, the environment
they were built from, size, how many secrets they get, and the SSH host to
connect to (ssh lima-<name>).

Other Lima instances aren't shown. ls also tidies up after VMs deleted with
"limactl delete": their Claude Desktop entries and saved state are removed.

With --json it prints a JSON array instead, one object per VM, and notes
about tidying up go to stderr. An environment that isn't known is "".

```
limabean ls [flags]
```

### Examples

```
  limabean ls

  # Plain columns, for scripts
  limabean ls --plain | awk 'NR > 1 { print $1 }'

  # JSON, for scripts: the names of running VMs
  limabean ls --json | jq -r '.[] | select(.status == "Running") | .name'
```

### Options

```
  -h, --help   help for ls
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

