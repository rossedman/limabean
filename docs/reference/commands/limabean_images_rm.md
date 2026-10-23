## limabean images rm

Delete images

### Synopsis

Delete images to free their disk space. VMs copied from them aren't
affected; the next VM of that kind is made the slow way, and builds a new
image in the background.

```
limabean images rm <image>... | --all [flags]
```

### Examples

```
  limabean images rm limabean-img-3fa2c1d0-t2k9x1
  limabean images rm --all
```

### Options

```
      --all    delete every image
  -h, --help   help for rm
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

* [limabean images](limabean_images.md)	 - Manage the images that make new VMs start in seconds

