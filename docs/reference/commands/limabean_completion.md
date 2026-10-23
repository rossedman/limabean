## limabean completion

Print a shell completion script

### Synopsis

Print a script that teaches your shell to tab-complete limabean's commands,
flags and VM names.

Load it once to try it, or save it where your shell loads completions from
to keep it.

```
limabean completion <bash|zsh|fish|powershell>
```

### Examples

```
  # zsh: try it now
  source <(limabean completion zsh)

  # zsh: keep it (any directory on your fpath works)
  limabean completion zsh > "${fpath[1]}/_limabean"

  # bash with bash-completion installed
  limabean completion bash > $(brew --prefix)/etc/bash_completion.d/limabean

  # fish
  limabean completion fish > ~/.config/fish/completions/limabean.fish
```

### Options

```
  -h, --help   help for completion
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

