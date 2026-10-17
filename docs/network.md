# Sandboxes: network allowlists

A VM can be limited to the domains its work needs, and locked so that
nothing inside it can lift the limit. That makes a VM a sandbox you can hand
to an agent: it can build, test and push to GitHub, but it can't reach
anything you didn't allow.

```bash
# GitHub and PyPI only, and no way to undo that from inside
limabean new agent --allow github --allow python --locked

limabean network ls agent                        # what it can reach
limabean network allow agent registry.npmjs.org  # one more domain
limabean network rm agent python                 # one fewer
```

Claude can make the same kind of VM for itself through
[MCP](mcp.md#sandboxes-for-a-task): `create_vm` takes `allow` and `locked`.

## What's allowed

Everything not on the allowlist is blocked, for the VM and for its Docker
containers. A domain allows its subdomains, so `github.com` covers
`api.github.com`. Names that aren't allowed don't resolve at all, so data
can't leave through DNS lookups either.

Presets name common sets:

| Preset | Domains |
|---|---|
| `claude` | anthropic.com, claude.ai, claude.com |
| `docker` | docker.io, docker.com |
| `github` | github.com, githubusercontent.com, githubassets.com, ghcr.io |
| `go` | go.dev, golang.org, proxy.golang.org, sum.golang.org |
| `java` | maven.org, apache.org, gradle.org, gradle-dn.com |
| `npm` | npmjs.org, npmjs.com, yarnpkg.com |
| `python` | pypi.org, pythonhosted.org |
| `ruby` | rubygems.org, ruby-lang.org |
| `rust` | crates.io, rust-lang.org |
| `ubuntu` | ubuntu.com |

`ntp.ubuntu.com` is always allowed alongside an allowlist, for the clock.

Installers often fetch from a second domain, such as a CDN. When something
fails with `Could not resolve host`, the missing name is in the error; add it
with `limabean network allow`.

## Lock it, or it's only a guardrail

A Lima VM's user has passwordless `sudo`, and Docker's socket is
root-equivalent. So in an unlocked VM, anything running as you, an agent
included, could turn the allowlist off. An unlocked allowlist still stops
accidents, but it isn't a boundary.

**Locking** takes those away from the VM's user: passwordless `sudo`, the
`sudo` and `docker` groups, and the Docker socket. The lock is re-applied on
every boot.

```bash
limabean new agent --allow github --locked   # locked from the start
limabean network lock dev                    # or later
limabean network unlock dev                  # sudo now, Docker on restart
```

Lock a VM before giving it to an agent. A root process started before the
lock keeps running until the VM restarts.

A locked VM has no `docker` for its user, and nothing that needs `sudo`
works. It suits agents that build and test code; for container work, leave
the VM unlocked, or use a separate one.

## Commands

| Command | Does |
|---|---|
| `limabean new <name> --allow <x>… [--locked]` | Creates a VM with an allowlist, locked or not |
| `limabean network ls [name]` | Shows the policy |
| `limabean network allow <name> <x>…` | Adds domains or presets; an open VM gets an allowlist of just these |
| `limabean network rm <name> <x>…` | Removes some; it won't remove the last one |
| `limabean network open <name>` | Removes the allowlist; a locked VM stays locked |
| `limabean network lock <name>` / `unlock` | Takes sudo and Docker away, or gives them back |

`limabean ls` shows each VM's network in its `NETWORK` column.

## How it works

The policy is a small file on your Mac,
`~/.config/limabean/network/<vm>/network.conf`, mounted **read-only** into
the VM at `/mnt/host/limabean`. Nothing inside the VM can edit it.
`limabean network` changes the file and asks the VM to apply it, using the one
`sudo` command a locked VM's user is still allowed.

Inside the VM, `limabean-network` applies it:

- **dnsmasq** answers only allowed domains; everything else is NXDOMAIN. It
  adds the addresses it returns to nftables sets.
- **nftables** drops every other packet leaving the VM or its containers.
  Only dnsmasq may reach the upstream DNS server.
- Containers resolve through dnsmasq too, on the `docker0` address.

The first boot installs the toolchain with the network open; the policy
applies when `limabean new` or `rebuild` finishes. Later boots apply it as the
VM starts. If the policy can't be read, nothing is changed: a missing file is
never taken to mean "open".

VMs created before limabean had this need `limabean rebuild` first. The
design, and what it would take to enforce this on the Mac instead, is in
[design note D15](design.md).
