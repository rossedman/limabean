## limabean version

Print limabean's version

### Synopsis

Print limabean's version, and the Go version and platform it was built for.

Release builds carry their tag; builds from "go install" report the module
version; local builds report "dev". With --json it prints them as a JSON object.

```
limabean version [flags]
```

### Examples

```
  limabean version
  limabean --version
  limabean version --json | jq -r .version
```

### Options

```
  -h, --help   help for version
      --json   print a JSON object, for scripts
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

