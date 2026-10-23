## limabean autostop status

Show whether idle VMs are stopped automatically

### Synopsis

Show whether the autostop agent is installed and loaded, and after how
long an unused VM is stopped. To see what it would do right now, run
"limabean down --idle <after> --preview".

```
limabean autostop status [flags]
```

### Examples

```
  limabean autostop status
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

* [limabean autostop](limabean_autostop.md)	 - Stop VMs automatically when nobody is using them

