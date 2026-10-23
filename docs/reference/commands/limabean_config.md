## limabean config

Show your settings and where each comes from

### Synopsis

Show every setting, its value, and where the value comes from: a flag, a
LIMABEAN_* environment variable, the config file, or limabean's default.
Earlier ones win, in that order.

The config file is written by "limabean init"; edit it, or run init again,
to change a setting.

```
limabean config [flags]
```

### Examples

```
  limabean config

  # One setting, for scripts
  limabean config --json | jq -r '.[] | select(.name == "CODE_DIR") | .value'
```

### Options

```
  -h, --help   help for config
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

