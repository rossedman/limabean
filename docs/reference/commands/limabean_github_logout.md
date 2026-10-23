## limabean github logout

Remove the GitHub token from the Keychain and VMs

### Synopsis

Delete the GitHub token from your Keychain, remove the GITHUB_TOKEN secret
that points at it, and update running VMs, so git there goes back to
pushing over SSH.

This doesn't revoke the token: do that on GitHub too.

```
limabean github logout [flags]
```

### Examples

```
  limabean github logout
```

### Options

```
  -h, --help   help for logout
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

