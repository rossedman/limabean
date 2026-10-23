## limabean images

Manage the images that make new VMs start in seconds

### Synopsis

New VMs are copies of an image: a VM that has already booted, installed
every update, Docker and its packs' tools, and then shut down. Copying it
is an APFS clone, so "limabean up", "run" and "rebuild" take
seconds instead of minutes.

There's an image for each environment and set of packs, built from the
current templates. The first VM of a kind is made the slow way, and its
image is built in the background for the next one. An image more than a
week old is still used, and replaced in the background. When the templates
or packs change, the next VM is made the slow way and a new image is built.

Images are hidden from "limabean ls". Each takes a few GiB, shared with the
VMs copied from it until they change. IMAGES="no" in the config turns them
off.

### Examples

```
  limabean images ls

  # Build one now, so even the first VM of a kind is fast
  limabean images build --with rust

  limabean images prune
  limabean images rm --all
```

### Options

```
  -h, --help   help for images
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
* [limabean images build](limabean_images_build.md)	 - Build an image now
* [limabean images ls](limabean_images_ls.md)	 - List images
* [limabean images prune](limabean_images_prune.md)	 - Delete images that are no longer used
* [limabean images rm](limabean_images_rm.md)	 - Delete images

