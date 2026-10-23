## limabean network allow

Let a VM reach more domains

### Synopsis

Add domains or presets to a VM's allowlist. A VM with an open network gets
an allowlist of just these, so everything else is blocked from then on.

A domain allows its subdomains: github.com covers api.github.com. Presets:

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

```
limabean network allow <name> <domain|preset>... [flags]
```

### Examples

```
  limabean network allow agent github python
  limabean network allow agent registry.example.com
```

### Options

```
  -h, --help   help for allow
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

* [limabean network](limabean_network.md)	 - Limit which hosts a VM can reach, and lock it down

