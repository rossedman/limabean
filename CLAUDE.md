# limabean

A Go CLI that creates disposable Ubuntu dev VMs on macOS from layered Lima
templates, drives `limactl`, and exposes VMs to Claude (Desktop SSH entries
and an MCP server). Docs: `docs/` (built with MkDocs to GitHub Pages).

## Layout

```
cmd/limabean/        entry point
internal/cli/        Cobra commands, one per file; vm.go and secrets_flow.go hold shared helpers;
                     mcp.go is the MCP server; images.go the image cache
internal/config/     Viper settings (~/.config/limabean/config, LIMABEAN_*, flags)
internal/lima/       limactl and ssh runner; never imports Lima's Go packages
internal/project/    .limabean.yaml for `limabean up`
internal/templates/  the templates directory, packs and environments
internal/pins/       pinned versions in mise TOML, for `limabean bump`
internal/atomic/     atomic writes for limabean's own files (config, secrets, state)
templates/           Lima templates, embedded in the binary: layer-os, layer-base,
                     environments, features/*.sh, files/, packs/
recipes/             starter .limabean.yaml files for `limabean recipes`, embedded;
                     TestRecipesBringVMsUp runs `up` on every one
docs/, overrides/    the MkDocs site; mkdocs.yml is its nav
```

`docs/go-cli.md` and `docs/design.md` explain the architecture and the
decisions (D1–D33); read the relevant note before changing a design.

## Checks

Run these before pushing; CI runs the same (`.github/workflows/lint.yml`):

```bash
gofmt -l .                       # must print nothing
go vet ./...
go test ./...                    # CI also fails below 90% total coverage:
go test -coverprofile=/tmp/c.out ./... && go tool cover -func=/tmp/c.out | tail -1
shellcheck templates/features/*.sh templates/files/limabean-network
shellcheck -s sh templates/files/env.sh templates/files/limabean-github-git templates/files/limabean-info
actionlint                       # when .github/ changes
mkdocs build --strict -d /tmp/limabean-site   # when docs/, overrides/ or mkdocs.yml change
                                 # (pip install -r docs/requirements.txt)
```

- Build the docs **outside the repo** (`-d`): a committed `site/` has
  happened before.
- Tests that check permission errors fail when run as root, as in cloud
  containers: `TestLoadErrors`, `TestSaveErrorsAndYes` (config),
  `TestFileErrors` (secrets), `TestRootErrors`, `TestUserDirErrors`,
  `TestCopyUserEdgeCases` (templates), `TestUnreadableAndUnwritable`
  (claude). That's expected there; anything else failing is real.
- `limactl tmpl validate` only runs on macOS CI; template changes can't be
  validated in a Linux container.

## Working here

- Path-scoped rules in `.claude/rules/` load when you touch those files:
  Go CLI, templates, docs, workflows. `.claude/rules/security.md` always
  applies.
- A behavior change isn't done until its docs are: the page in `docs/`,
  `docs/cli.md`, and the README command table when commands change.
- `docs/limabean.schema.json` is generated: after changing the project file,
  a pack or a network preset, run
  `go test ./internal/cli -run TestSchema -update-schema`.
- Prefer small, verified changes. If something can't be checked here (it
  needs a Mac, Lima or a real VM), say so in the PR's test plan as an
  unchecked item.
- Commits: an imperative subject, a body saying why, and a bulleted list of
  what changed for larger commits.
