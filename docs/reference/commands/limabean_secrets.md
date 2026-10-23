## limabean secrets

Manage the environment variables and secrets VMs get

### Synopsis

Secrets are environment variables limabean installs in VMs: API keys,
tokens, or plain settings. On your Mac they're kept as references, never as
values, in two kinds of files with one NAME=reference per line:

  ~/.config/limabean/secrets.env         every VM
  ~/.config/limabean/secrets.<vm>.env    one VM; overrides the shared file

A reference says where the value comes from:

  keychain:<service>      an item in your macOS Keychain
  op://vault/item/field   a 1Password item, read with the op CLI
  env:<VAR>               a variable in the shell you run limabean in
  cmd:<command>           a command's output, run with sh on the Mac
  anything else           used as is (for settings, never for secrets)

"limabean secrets sync" looks each one up on the Mac and writes the values
to ~/.config/limabean/secrets.env (mode 600) in each VM, where every shell
loads them. New VMs get their secrets automatically.

Anything in a VM, Claude included, can read its secrets: prefer narrow,
short-lived credentials, and give a VM only what it needs.

### Examples

```
  # Paste a value now; it's saved in your Keychain
  limabean secrets set OPENAI_API_KEY

  # Point at 1Password instead, for one VM only
  limabean secrets set NPM_TOKEN op://Work/npm/token --vm dev

  # See what each VM gets, then push changes to running VMs
  limabean secrets ls
  limabean secrets sync
```

### Options

```
  -h, --help   help for secrets
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
* [limabean secrets edit](limabean_secrets_edit.md)	 - Edit a secrets file in your editor
* [limabean secrets ls](limabean_secrets_ls.md)	 - List secrets and where their values come from
* [limabean secrets rm](limabean_secrets_rm.md)	 - Remove a secret
* [limabean secrets set](limabean_secrets_set.md)	 - Add or change a secret
* [limabean secrets sync](limabean_secrets_sync.md)	 - Install secrets in running VMs

