## limabean open

Open a port a VM forwards in your browser

### Synopsis

Open http://localhost:<port> in your default browser, for a dev server or
container running in a VM (see "limabean ports").

Leave out the port and the VM's only forwarded port is opened, or you pick
one from a list. Leave out both to pick the VM too. A single number is
taken as the port when it isn't a VM's name.

```
limabean open [name] [port] [flags]
```

### Examples

```
  limabean open dev 3000

  # dev's only forwarded port, or pick one
  limabean open dev

  # Port 8080 of the only VM, or pick the VM
  limabean open 8080
```

### Options

```
  -h, --help   help for open
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

