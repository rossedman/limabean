## limabean secrets set

Add or change a secret

### Synopsis

Add a secret, or change where an existing one comes from, then install it in
running VMs.

With a reference (keychain:…, op://…, env:…, cmd:… or a plain value), it's
written as given. Without one, you're asked for the value, which is saved in
your Keychain as limabean-secret-NAME (limabean-secret-<vm>-NAME with --vm)
and referenced from there; --from-stdin reads the value from stdin instead.
With no arguments at all, a form asks for everything.

The secret goes to every VM, or with --vm to that VM only.

```
limabean secrets set [NAME] [reference] [flags]
```

### Examples

```
  # Ask for everything
  limabean secrets set

  # Paste the value at a hidden prompt; saved in the Keychain
  limabean secrets set OPENAI_API_KEY

  # From a password manager or script, without a prompt
  op read op://Private/OpenAI/credential | limabean secrets set OPENAI_API_KEY --from-stdin

  # References: 1Password, an environment variable, a command
  limabean secrets set ANTHROPIC_API_KEY op://Private/Anthropic/credential
  limabean secrets set NPM_TOKEN env:NPM_TOKEN --vm web
  limabean secrets set VAULT_TOKEN 'cmd:vault print token'

  # A plain setting
  limabean secrets set AWS_REGION us-east-1
```

### Options

```
      --from-stdin   read the value from stdin instead of prompting
  -h, --help         help for set
      --no-sync      don't install it in running VMs yet
      --vm string    only for this VM (default: every VM)
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

