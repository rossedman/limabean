## limabean network

Limit which hosts a VM can reach, and lock it down

### Synopsis

Give a VM an allowlist of domains it may reach; everything else is
blocked, for the VM and its containers. A domain allows its subdomains, and
presets name common sets:

  aws      amazonaws.com, awsapps.com
  azure    azure.com, microsoft.com, microsoftonline.com, windows.net, azureedge.net
  claude   anthropic.com, claude.ai, claude.com
  docker   docker.io, docker.com
  dotnet   nuget.org, dot.net, microsoft.com
  elixir   hex.pm, hexdocs.pm
  gcp      googleapis.com, google.com, gstatic.com
  github   github.com, githubusercontent.com, githubassets.com, ghcr.io
  go       go.dev, golang.org, proxy.golang.org, sum.golang.org
  java     maven.org, apache.org, gradle.org, gradle-dn.com
  npm      npmjs.org, npmjs.com, yarnpkg.com
  python   pypi.org, pythonhosted.org
  ruby     rubygems.org, ruby-lang.org
  rust     crates.io, rust-lang.org
  ubuntu   ubuntu.com

Locking a VM also takes sudo and Docker away from its user. Without that,
anything running in the VM, an agent included, could become root and turn
the allowlist off; with it, the allowlist holds. Locked VMs suit agents
that should only reach what their task needs.

The policy is kept on your Mac (~/.config/limabean/network/<vm>/) and
mounted read-only into the VM, so nothing inside can change it. Changes
apply to a running VM at once, and to a stopped one when it starts.
Disallowed names don't resolve, so data can't leave through DNS either.
VMs created before this feature need a rebuild first.

### Examples

```
  # A sandbox that can reach GitHub and PyPI, and can't undo that
  limabean up agent --allow github --allow python --locked

  limabean network ls agent
  limabean network allow agent registry.npmjs.org
  limabean network rm agent python
  limabean network open agent
  limabean network unlock agent

  # Find out what an allowlist needs: record, work, then allow what was used
  limabean network learn agent
  limabean network log agent
  limabean network learn agent --done
```

### Options

```
  -h, --help   help for network
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
* [limabean network allow](limabean_network_allow.md)	 - Let a VM reach more domains
* [limabean network learn](limabean_network_learn.md)	 - Record what a VM reaches, to build its allowlist
* [limabean network lock](limabean_network_lock.md)	 - Take sudo and Docker away from a VM's user
* [limabean network log](limabean_network_log.md)	 - Show the names a VM looked up, and which were blocked
* [limabean network ls](limabean_network_ls.md)	 - Show a VM's network policy
* [limabean network open](limabean_network_open.md)	 - Let a VM reach any host again
* [limabean network rm](limabean_network_rm.md)	 - Stop a VM reaching some domains
* [limabean network unlock](limabean_network_unlock.md)	 - Give a VM's user sudo and Docker back

