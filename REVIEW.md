# Review checklist

Read with `CLAUDE.md` and `.claude/rules/`. Flag a change that:

## Correctness

- Calls `exec` (or `os/exec`) directly from a command instead of a
  replaceable `App` field or `a.Lima`, so tests can't cover it.
- Adds a command without `Short`, `Long` and `Example`, or puts two commands
  in one file.
- Prompts or opens a form without checking `a.UI.Prompting`, which would hang
  scripts and MCP.
- Changes `--json` or plain table output that scripts read, without saying so.
- Uses a `limactl` flag or command newer than the minimum Lima version
  (2.0.0) without a fallback.
- Adds a provisioning script step that isn't idempotent, or is slow on a
  boot with nothing to do (scripts run on every boot, also in image copies).
- Adds code without tests for its failure paths (CI requires 90% coverage).

## Security

See `.claude/rules/security.md`. Always flag:

- Input from MCP, a `.limabean.yaml`, a VM or the network reaching a command
  on the Mac, a secret reference, or a file path on the Mac.
- A secret value in argv, logs, errors, MCP results or `--json`.
- Any path by which a VM, MCP or a project file could widen a network
  allowlist or lift a lock.
- A destructive action losing its guard (unsaved-work check, confirmation,
  `--allow-destructive`).
- A name reaching a command line unvalidated.

## Docs

- Behavior changes without matching updates to `docs/`, `docs/cli.md`, the
  README command tables or the landing page's command list.
- Docs claiming something the code doesn't do.
- A new docs page missing from `mkdocs.yml`'s nav.

## Not worth flagging

- Style that `gofmt` decides.
- Long `Long` help texts; they're meant to be complete.
