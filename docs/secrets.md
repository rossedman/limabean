# Secrets

Secrets are environment variables limabean installs in VMs: API keys,
tokens, or plain settings. Every shell in the VM has them, and so does
Claude when it works there over SSH.

On your Mac, limabean stores **references** rather than values: where each
value comes from. Values are looked up when secrets are synced and written
only inside VMs.

## Adding secrets

The easiest way is while creating a VM. In a terminal, `limabean new` shows
a secrets step:
- The **shared** secrets are listed. Every VM gets them.
- **Secrets from your other VMs** can be added to this one.
- **Add a secret** asks for a name and where the value comes from: paste it
  now (saved in your Keychain), a 1Password item, an environment variable on
  your Mac, or a command. Then it asks whether the secret is for this VM or
  every VM.

Any time later:

```bash
limabean secrets set OPENAI_API_KEY                        # paste the value; saved in the Keychain
limabean secrets set ANTHROPIC_API_KEY op://Private/Anthropic/credential
limabean secrets set NPM_TOKEN env:NPM_TOKEN --vm web      # only for the web VM
limabean secrets set VAULT_TOKEN 'cmd:vault print token'
limabean secrets set AWS_REGION us-east-1                  # a plain setting
op read op://Private/OpenAI/credential | limabean secrets set OPENAI_API_KEY --from-stdin
```

`set`, `rm` and `edit` update running VMs straight away; stopped ones get
the change at their next `limabean secrets sync`. From scripts,
`limabean new <name> --secret NAME=reference` gives a new VM its own
secrets.

## Where they're kept

```
~/.config/limabean/secrets.env         every VM
~/.config/limabean/secrets.<vm>.env    one VM; overrides the shared file
```

One `NAME=reference` per line (mode 600). Edit them directly with
`limabean secrets edit [--vm <name>]`:

| Reference | Value comes from |
|---|---|
| `keychain:<service>` | an item in your macOS login Keychain. `secrets set` creates `limabean-secret-<NAME>`, or `limabean-secret-<vm>-<NAME>` for one VM |
| `op://vault/item/field` | 1Password, through the [op CLI](https://developer.1password.com/docs/cli/) (`brew install 1password-cli`) |
| `env:<VAR>` | a variable in the shell you run limabean in |
| `cmd:<command>` | a command's output, run with `sh` on your Mac |
| anything else | the text itself, for settings only, never for secrets |

`limabean secrets ls` shows each secret's source and which VMs get it, and
never shows a value. `limabean secrets ls --vm dev` shows exactly what `dev`
gets.

## In the VM

`limabean secrets sync` looks every value up on the Mac and writes
`~/.config/limabean/secrets.env` in each VM: `export NAME='value'` lines,
mode 600, replacing what was there. A secret you removed disappears too.
Every shell loads that file, interactive or not.

A reference that can't be looked up, such as a missing Keychain item, a
locked 1Password or an unset variable, is reported and left out, and the
rest are still installed. `limabean doctor` checks every reference without
reading any values, so 1Password never prompts.

## Choosing what to share

Anything in a VM can read its secrets, Claude included. Keep each VM to what
it needs:
- Put a secret in `secrets.<vm>.env` (`--vm`), not the shared file, when
  only one VM uses it.
- Prefer short-lived and narrowly scoped credentials: fine-grained GitHub
  tokens with an expiry, AWS SSO sessions, or API keys limited to one
  project.
- Rotate by changing the source, for example running `secrets set` again or
  updating the 1Password item, then run `limabean secrets sync`.

`limabean rm` deletes a VM's own secrets file, and any Keychain items limabean
created for it that nothing else uses. Shared secrets are kept.
