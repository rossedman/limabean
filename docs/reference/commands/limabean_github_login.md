## limabean github login

Save a GitHub token and install it in VMs

### Synopsis

Check a GitHub token with GitHub, save it to your Keychain, make it the
GITHUB_TOKEN secret for every VM, and install it in running VMs. New VMs
get it automatically.

In a terminal you're asked for the token at a hidden prompt. With
--with-token (like "gh auth login --with-token") it's read from stdin, for
scripts and password managers. A token that isn't fine-grained is accepted
with a warning, since classic and gh tokens usually have broad scopes.

Run it again with a new token when the old one expires.

```
limabean github login [flags]
```

### Examples

```
  # Paste a token
  limabean github login

  # From 1Password
  op read "op://Private/GitHub limabean/token" | limabean github login --with-token

  # Reuse your host gh login
  gh auth token --hostname github.com | limabean github login --with-token
```

### Options

```
  -h, --help         help for login
      --no-sync      don't install it in running VMs yet
      --with-token   read the token from stdin
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

