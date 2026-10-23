# Contributing

Thanks for helping with limabean. Issues and pull requests are welcome:
bugs, docs, new packs and recipes, features.

For a security problem, don't open an issue: see [SECURITY.md](SECURITY.md).

## Before you start

- **Bugs**: open an [issue](https://github.com/rossedman/limabean/issues/new/choose)
  with `limabean version`, `limactl --version`, your macOS version, and
  `limabean doctor --plain` if it's about your setup.
- **Features**: open an issue first for anything bigger than a small fix,
  so we can agree on the shape before you write it. The
  [design notes](docs/design.md) (D1 onward) explain why limabean works the
  way it does; a change that goes against one should say why.
- **Packs and recipes** are the easiest place to start: a pack is a mise TOML
  file in `templates/packs/`, a recipe a commented `.limabean.yaml` in
  `recipes/`.

## Setting up

You need Go (the version in `go.mod`), and for the checks golangci-lint,
shellcheck, actionlint and Python for the docs:

```bash
brew install go golangci-lint shellcheck actionlint
git clone https://github.com/rossedman/limabean && cd limabean
go build -o limabean ./cmd/limabean
./limabean --help
```

Most of limabean is tested without Lima: the tests use a fake `limactl`,
ssh, Keychain and launchd. To try a change for real you need a Mac with
Lima 2.0 or newer (`brew install lima`).

`docs/go-cli.md` explains the code's layout, and `CLAUDE.md` is a short map
of the repository.

## Checks

Run `make check` before you push. CI runs the same checks and fails a pull
request on any of them:

| Target | What it checks |
|---|---|
| `make lint` | gofmt, `go vet`, golangci-lint, a doc comment on every function (`tools/doccheck`), shellcheck and actionlint |
| `make cover` | `go test -race`, with total coverage at 90% or more |
| `make docs` | regenerates `docs/reference/commands` from the commands' help; commit what changes |
| `make docs-build` | `mkdocs build --strict`, outside the repository |
| `make check` | all of the above, every benchmark once, and govulncheck |
| `make bench` | the benchmarks, six runs each into `bench.out`, for `benchstat` |
| `make fuzz` | every fuzz target for 30 seconds (`FUZZTIME=5m` for longer); CI does the same |

You need `golangci-lint`, `shellcheck` and `actionlint`
(`brew install golangci-lint shellcheck actionlint`), and the docs packages:
`pip install -r docs/requirements.txt`.

- Build the docs outside the repository (`-d`), so `site/` isn't committed.
- `docs/limabean.schema.json` is generated: after changing the project file,
  a pack or a network preset, run
  `go test ./internal/cli -run TestSchema -update-schema`.
- Template changes are validated with `limactl tmpl validate`, which CI runs
  on macOS.

## Writing a change

- **Tests**: cover the failure paths, not only the happy one. Command tests
  go through the harness in `internal/cli/cli_test.go`.
- **Docs**: a behavior change isn't done until its docs are: the page in
  `docs/`, `docs/cli.md`, and the README's command table when commands
  change. Write plainly, with commands people can paste.
- **Security**: limabean keeps the rules in
  [`.claude/rules/security.md`](.claude/rules/security.md). If your change
  touches one (input that reaches the Mac, secrets, network policy,
  destructive actions), say how in the pull request and add a test for the
  refusal.
- **Commits**: an imperative subject ("Add a zig pack"), and a body that
  says why; a bulleted list of what changed for larger ones.
- **Pull requests**: the template asks why, what changed, security, and a
  test plan. List what you couldn't check (it needs a Mac, Lima or a real
  VM) as unchecked items.

[`REVIEW.md`](REVIEW.md) is the checklist reviews use.

## License

By contributing, you agree your contribution is licensed under the
[MIT License](LICENSE), like the rest of limabean.
