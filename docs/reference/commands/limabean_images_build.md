## limabean images build

Build an image now

### Synopsis

Build the image new VMs of an environment and set of packs are copied
from, replacing an older one. VMs get the PACKS setting's packs (none
besides core, unless you set it) with --with and --without as for
"limabean up <name>", or exactly the packs in --packs.

limabean builds images in the background by itself; build one by hand so
that even the first VM of a kind starts in seconds, or to pick up updates
before the week is up.

```
limabean images build [flags]
```

### Examples

```
  # The image "limabean up <name>" uses
  limabean images build

  # The one "limabean up api --with rust" uses
  limabean images build --with rust
```

### Options

```
      --env string        environment template (default "scratch")
  -h, --help              help for build
      --if-needed         do nothing if the image exists and is under a week old, or is being built
      --packs string      exactly these packs besides core, e.g. go,node
      --with strings      add these packs to the PACKS setting's
      --without strings   leave these packs out
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

