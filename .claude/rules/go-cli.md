---
paths:
  - "cmd/**"
  - "internal/**"
  - "templates.go"
  - "go.mod"
---

# Go CLI

## Commands

- One `cobra.Command` constructor per file (`newXxxCmd(a *App)`), registered
  in `root.go` or its group's parent file. `TestOneCommandPerFile` enforces it.
- Every command has `Short`, `Long` and `Example`, written for someone new to
  limabean; `TestEveryCommandIsDocumented` enforces it.
- Settings go through Viper (`internal/config`): a key, a default in `New()`,
  a field on `Config`, a line in `Save()`'s commented file, and a flag bound
  with `a.V.BindPFlag` where there is one. Document it in `docs/cli.md`'s
  settings block.
- Plain mode (`--plain`, no terminal) never prompts: missing input is an
  error, questions take their defaults. Check `a.UI.Prompting` before any
  form or confirmation.
- Output that scripts read (`--json`, plain tables) stays stable; notes go to
  stderr or only appear when prompting.

## Outside effects

- Everything that touches the world goes through a replaceable field on
  `App` (`execSSH`, `attachSSH`, `launchctl`, `openEditor`, `cloneFile`,
  `miseLatest`, `macListens`, `openURL`, `spawn`) or through `a.Lima`'s
  `Runner`. Add a field rather than calling `exec` directly.
- `internal/lima` drives `limactl`; it never imports Lima's Go packages.
  New limactl features need the minimum version (`minimumLimaVersion` in
  `templates/layer-os.yaml`, 2.0.0) or a fallback.
- Secret values never go in argv: pass them on stdin.

## Tests

- Command tests run through the harness in `internal/cli/cli_test.go`:
  `newHarness(t)`, then `h.must(stdin, args...)` / `h.run(...)` /
  `h.runTTY(keys, args...)`. `fakeWorld` stands in for limactl, ssh, the
  Keychain and launchd; extend it (a field and a `case`) for new limactl
  calls rather than mocking elsewhere.
- The harness sets `LIMABEAN_IMAGES=no`; image tests turn it on.
- Assert on recorded calls (`h.world.called(prefix)`, `h.world.calls`) and on
  output, and cover the failure paths: CI fails below 90% coverage.
- `gofmt` runs on every edit through a hook; still run `go vet ./...`.

## MCP (`internal/cli/mcp.go`)

- Tools run limabean's own commands in-process (`runCLI`), so every guard
  applies. Validate every name with `checkNames` and pass values as
  `--flag=value`, so input can never become a flag.
- Nothing may run on the Mac or reveal a secret value.
- No tool may widen a VM's network or lock; tools that destroy work exist
  only with `--allow-destructive` and carry
  `_meta["anthropic/requiresUserInteraction"]`.
- Add new tools to the expectations in `mcp_test.go` and to `docs/mcp.md`'s
  tools table.
