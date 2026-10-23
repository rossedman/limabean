## limabean secrets edit

Edit a secrets file in your editor

### Synopsis

Open the shared secrets file, or with --vm that VM's own file, in $VISUAL or
$EDITOR (vi if neither is set). The file is created with a short header if
it doesn't exist yet.

Afterwards the file is checked, and, in a terminal, you're offered to
install the changes in running VMs.

```
limabean secrets edit [flags]
```

### Examples

```
  limabean secrets edit

  EDITOR="code --wait" limabean secrets edit --vm dev
```

### Options

```
  -h, --help        help for edit
      --vm string   edit this VM's own file
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

* [limabean secrets](limabean_secrets.md)	 - Manage the environment variables and secrets VMs get

