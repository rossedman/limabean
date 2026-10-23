## limabean github

Give VMs a GitHub token for gh, the API and https pushes

### Synopsis

Manage the GitHub token VMs use. It's stored in your macOS Keychain
(limabean-github-token) and installed in VMs as the GITHUB_TOKEN secret, so
it's one of your secrets, with a few shortcuts here.

With a token, git in VMs talks to GitHub over https with it, including for
SSH-style remotes, so pushing works in Claude Desktop sessions, which have no
forwarded ssh-agent. Without one, git pushes over SSH with your agent.

Use a fine-grained token limited to the repositories you'll work on, with an
expiry and only the permissions you need (Contents and Pull requests: read
and write is typical). Anything in a VM, Claude included, can use it.

### Examples

```
  # Paste a fine-grained token
  limabean github login

  # Reuse your gh login (broader scopes; limabean warns)
  gh auth token --hostname github.com | limabean github login --with-token

  limabean github status
```

### Options

```
  -h, --help   help for github
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
* [limabean github login](limabean_github_login.md)	 - Save a GitHub token and install it in VMs
* [limabean github logout](limabean_github_logout.md)	 - Remove the GitHub token from the Keychain and VMs
* [limabean github status](limabean_github_status.md)	 - Show the GitHub token's account and expiry, and which VMs have it
* [limabean github sync](limabean_github_sync.md)	 - Install the GitHub token in every running VM

