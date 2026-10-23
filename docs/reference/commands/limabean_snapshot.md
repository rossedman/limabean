## limabean snapshot

Save a VM's state and roll back to it later

### Synopsis

Save a VM's disks as a snapshot, and restore one to put the VM back exactly
as it was: the system, and ~/code when it's on its own disk. Take one before
letting Claude loose on something risky, and roll back if it goes wrong.

Snapshots are APFS clones of the VM's disks, so they take a few seconds and
only use space for what changes afterwards. The VM is shut down while one
is saved or restored, and started again if it was running.

Snapshots live with the VM: rm and rebuild delete them.

### Examples

```
  limabean snapshot save dev before-refactor
  limabean snapshot ls dev
  limabean snapshot restore dev before-refactor
  limabean snapshot rm dev before-refactor
```

### Options

```
  -h, --help   help for snapshot
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
* [limabean snapshot ls](limabean_snapshot_ls.md)	 - List a VM's snapshots
* [limabean snapshot restore](limabean_snapshot_restore.md)	 - Put a VM back the way a snapshot saved it
* [limabean snapshot rm](limabean_snapshot_rm.md)	 - Delete a snapshot
* [limabean snapshot save](limabean_snapshot_save.md)	 - Save a snapshot of a VM

