## limabean secrets rm

Remove a secret

### Synopsis

Remove a secret from the shared file, or with --vm from that VM's own file,
then update running VMs so it's gone from them too.

If the value lived in a Keychain item limabean created (limabean-secret-…)
and nothing else refers to it, the item is deleted as well. Items you
created yourself, 1Password items and the like are never touched.

```
limabean secrets rm NAME [flags]
```

### Examples

```
  limabean secrets rm OPENAI_API_KEY

  # Only dev's own copy
  limabean secrets rm NPM_TOKEN --vm dev
```

### Options

```
  -h, --help        help for rm
      --no-sync     don't update running VMs yet
      --vm string   remove it from this VM's own file
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

