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

You need Go (the version in `go.mod`), and for some checks shellcheck,
actionlint and Python for the docs:

```bash
brew install go shellcheck actionlint
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

Run these before you push; CI runs the same, and fails a pull request on
any of them:

```bash
gofmt -l .                       # must print nothing
go vet ./...
go test ./...                    # and total coverage must stay at 90% or more:
go test -coverprofile=/tmp/c.out ./... && go tool cover -func=/tmp/c.out | tail -1
shellcheck templates/features/*.sh templates/files/limabean-network
shellcheck -s sh templates/files/env.sh templates/files/limabean-github-git templates/files/limabean-info
actionlint                       # when .github/ changes
pip install -r docs/requirements.txt
mkdocs build --strict -d /tmp/limabean-site   # when docs/, overrides/ or mkdocs.yml change
```

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
