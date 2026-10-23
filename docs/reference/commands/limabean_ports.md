## limabean ports

List the ports a VM forwards to your Mac

### Synopsis

List the TCP ports programs in a VM are listening on, and whether each one
reaches your Mac. Lima forwards any port a program opens on localhost or on
all addresses in the VM (a dev server, or a container started with -p) to
the same port on the Mac's localhost, so http://localhost:3000 on the Mac
reaches port 3000 in the VM.

A port shows as not forwarded when nothing answers on the Mac's side, for
example because a program on the Mac already uses that port. Ports a
program opens on another address, like 127.0.0.53, aren't forwarded and
aren't listed, and neither is SSH.

Leave out the name to pick a VM from a list.

```
limabean ports [name] [flags]
```

### Examples

```
  limabean ports dev

  # Open one in the browser
  limabean open dev 3000

  # For scripts
  limabean ports dev --json | jq '.[] | select(.forwarded) | .port'
```

### Options

```
  -h, --help   help for ports
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

