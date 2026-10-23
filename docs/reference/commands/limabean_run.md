## limabean run

Run a command in a new VM, and delete it after

### Synopsis

Create a VM, run one command in it, and with --rm delete the VM when the
command ends, whether it worked or not. limabean exits with the command's
exit status.

--repo clones a repository first and runs the command in it. The VM gets
your shared secrets, the PACKS setting's packs (none besides core, unless
you set it) and an open network, unless flags say otherwise, as for
"limabean up <name>". Without --rm the VM is kept, named
run-<random> unless you name it; --ttl deletes it later.

A single argument is run by the shell as it is, so pipes and && work;
several arguments are quoted one by one.

```
limabean run [flags] -- <command>...
```

### Examples

```
  # A repository's tests in a clean VM, then delete it
  limabean run --rm --repo rossedman/limabean -- go test ./...

  # Claude Code on a task, in a locked sandbox that can only reach GitHub and Anthropic
  limabean run --rm --with claude --allow github --allow claude --locked \
    --repo me/app -- 'claude -p "make the tests pass" && git push'

  # Keep the VM for two hours afterwards
  limabean run --ttl 2h --name spike -- 'make bench | tee bench.txt'
```

### Options

```
      --allow stringArray    only let the VM reach this domain or preset (repeatable)
      --env string           environment template (default "scratch")
  -h, --help                 help for run
      --locked               take sudo and Docker away from the VM's user, so its allowlist holds
      --name string          name the VM (default run-<random>)
      --repo string          clone this repository first and run the command in it, e.g. owner/repo
      --rm                   delete the VM when the command ends
      --secret stringArray   give the VM a secret: NAME=reference (repeatable)
      --ttl duration         without --rm: delete the VM once it's this old, e.g. 2h
      --with strings         give the VM these packs, e.g. go,claude
      --without strings      leave these packs out
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

