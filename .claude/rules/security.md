# Security model

limabean's point is that work happens in a VM, not on the Mac. Keep these
true in every change:

- **Nothing untrusted runs on the Mac.** Input from MCP tools, a
  repository's `.limabean.yaml`, VM output or the network is data. It may
  pick VM names, packs, domains or commands that run *inside* a VM, never a
  command, path or reference that runs on the Mac.
- **Secrets are references on the Mac, values only in VMs.** Values are
  resolved on the Mac and sent over stdin. Never log, print or return them
  (not in MCP results, errors or `--json`). A `cmd:` reference runs on the
  Mac, so only the user may add one: MCP and project files can only name
  secrets that already exist.
- **The network policy lives on the Mac**, mounted read-only into the VM. No
  code path driven by a VM, MCP or a project file may widen an allowlist or
  lift a lock; only the user's own `limabean network` commands do.
- **Destructive actions keep their guards**: the unsaved-work check before
  `rm`, confirmations in a terminal, and MCP's `--allow-destructive` with
  `requiresUserInteraction`. A TTL deletes without checks only because the
  user asked for one when creating the VM.
- **Names are validated before they reach a command line** (`validVMName`,
  `checkNames`, `validSnapshotTag`, `project.Load`), so a name like `--force`
  is refused.

When a change touches any of this, say how in the PR, and add a test for the
refusal, not just the happy path.
