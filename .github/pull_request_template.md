## Why

<!-- The problem this solves, or the issue it closes. -->

## What changed

<!-- The behavior change, and the docs that cover it (docs/, docs/cli.md, the README's command table). -->

## Security

<!-- Does this touch input that reaches the Mac, secrets, a VM's network policy or a destructive action
     (.claude/rules/security.md)? If so, how, and which test covers the refusal. Otherwise: "None". -->

## Test plan

- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...` (coverage at 90% or more)
- [ ] `shellcheck` / `actionlint` / `mkdocs build --strict`, if those files changed
<!-- What you checked by hand. List what needs a Mac, Lima or a real VM as unchecked items. -->
