## limabean remote

Add a VM's clone of this repository as a git remote on your Mac

### Synopsis

Add a VM's clone of the repository you're in as a git remote of your
checkout on the Mac, so work done in the VM (by you or by Claude) can be
fetched and reviewed on the Mac without pushing it to GitHub first.

Run it in a checkout. With a .limabean.yaml, it uses the project's VM;
otherwise, or to use another VM, name one. The VM is started if it's
stopped, and must have cloned the repository (limabean up does).

The remote, called limabean unless you pass --name, points at
lima-<vm>:<clone> over SSH, the way limabean ssh and your editor reach the
VM, so it works whatever the VM's network allows. It fetches branches
only, not tags, so nothing in the VM can add tags to your checkout.
Fetching copies git objects into the Mac's checkout; it runs nothing from
the VM.

Run it again after a rebuild to point the remote at the new clone.

```
limabean remote [name] [flags]
```

### Examples

```
  # In a project's checkout: add the VM's clone, then fetch its branches
  limabean remote
  git fetch limabean
  git diff main...limabean/my-branch

  # Hand a branch to the VM
  git push limabean my-branch

  # Another VM, under another name
  limabean remote scratch --name scratch

  # Remove it
  limabean remote --rm
```

### Options

```
  -h, --help          help for remote
      --name string   the remote's name (default "limabean")
      --rm            remove the remote instead
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

