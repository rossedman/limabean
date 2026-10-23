## limabean github status

Show the GitHub token's account and expiry, and which VMs have it

### Synopsis

Check the saved GitHub token with GitHub, showing the account it belongs
to and when it expires, then list each VM: whether it has the token
installed, or is stopped.

A missing token in a running VM is fixed with "limabean github sync".

```
limabean github status [flags]
```

### Examples

```
  limabean github status
```

### Options

```
  -h, --help   help for status
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

* [limabean github](limabean_github.md)	 - Give VMs a GitHub token for gh, the API and https pushes

