# Security

limabean's point is that work happens in a VM, not on your Mac, so a way
around that is a security bug. Thanks for reporting one.

## Reporting a vulnerability

Report it privately through GitHub:
**[Report a vulnerability](https://github.com/rossedman/limabean/security/advisories/new)**
(the repository's Security tab → Advisories → Report a vulnerability).
Please don't open a public issue, pull request or discussion for it.

Include what you can of:

- what an attacker controls (a repository's `.limabean.yaml`, an MCP tool
  call, a VM, a network response) and what they get;
- steps or a proof of concept;
- limabean's version (`limabean version`), Lima's (`limactl --version`) and
  macOS's.

You'll get an answer within a week. A fix goes into a new release, and the
advisory is published with it, crediting you unless you'd rather not be.

## Supported versions

Fixes go into the latest release only. Update with
`brew upgrade limabean`.

## What counts

limabean keeps these true; a way to break one is a vulnerability:

- **Nothing untrusted runs on the Mac.** Input from MCP tools, a
  repository's `.limabean.yaml`, a VM's output or the network may choose
  what runs inside a VM, never a command, path or reference on the Mac.
- **Secret values stay out of sight.** They're resolved on the Mac and
  sent to VMs over stdin, never put in arguments, logs, errors, MCP results
  or `--json` output. Only you can add a `cmd:` reference, which runs on the
  Mac.
- **A VM's network policy is yours.** It lives on the Mac, mounted
  read-only into the VM. Nothing driven by a VM, an MCP tool or a project
  file may widen an allowlist or lift a lock.
- **Destructive actions keep their guards**: the unsaved-work check before
  `rm` and `rebuild`, confirmations in a terminal, and MCP's
  `--allow-destructive` with a confirmation for each call.
- **Your keys stay on the Mac.** VMs use your SSH agent through forwarding
  and a scoped GitHub token; git and AWS config are mounted read-only.

Not in scope:

- What runs inside a VM you created, with your approval: a project's
  `setup` and `start`, its tools, code you or Claude run there. A VM is yours
  to trust or throw away; locking its network is how you limit it.
- A VM whose network is open (the default) reaching the internet.
- Lima, macOS's Virtualization framework, Ubuntu or the tools packs install.
  Report those upstream; tell us too if limabean should work around one.
- Anything needing someone who already controls your Mac account.

The full model, and why each rule is there, is in
[docs/design.md](docs/design.md) and
[`.claude/rules/security.md`](.claude/rules/security.md).
