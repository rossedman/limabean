# The Go CLI

`limabean` is being rewritten from a bash script into a Go binary. The VM
templates don't change; only the tool that drives them does. This page is the
design. User-facing docs live in [cli.md](cli.md).

## Goals

- Behave like the bash CLI it replaces: same commands, config file, VM layout
  and Claude Desktop and Keychain conventions, so existing setups keep working.
- Make secrets easy to add, including in a single form when a VM is created.
- Ship as one binary (`brew install`, `go install`), with no runtime dependency
  on gum.
- Be tested: parsing, quoting and file edits have unit tests, and command
  wiring is checked automatically.

Non-goal: importing Lima's Go packages. `github.com/lima-vm/lima/v2` is
importable but makes no promise of a stable API, and it would tie limabean to
one Lima version while users run whatever `limactl` they have installed.
`limactl` is the boundary, and `limactl list --format json` gives structured
data.

## Layout

```
cmd/limabean/main.go       entry point: cli.Execute()
internal/cli/              Cobra commands, one per file (root.go, new.go, ls.go,
                           secrets_set.go, …); groups (github, secrets, claude,
                           network, snapshot, autostop, images) have a file for
                           the parent and one per subcommand; mcp.go is the MCP
                           server, which runs these commands in-process;
                           images.go builds and finds the images VMs are
                           copied from
internal/config/           Viper: config file, LIMABEAN_* environment, flags
internal/lima/             limactl runner (list, start, stop, delete, disks, ssh)
internal/pins/             pinned versions in mise TOML files, for bump
internal/project/          .limabean.yaml: find, parse and check, for up
internal/templates/        the templates directory, packs and environments
internal/secrets/          reference files: parse, merge, resolve, render
internal/keychain/         macOS Keychain via `security`, values on stdin
internal/claude/           sshConfigs entries in ~/.claude/settings.json
internal/github/           token checks against the GitHub API
internal/ui/               lipgloss styles, huh forms, spinners, plain mode
templates.go               embeds templates/ into the binary
```

## Conventions

- **One command per file.** Each file defines a single `cobra.Command`
  constructor such as `newNewCmd(app *App)`, and registers it with its parent
  in `root.go` or in the group's file.
- **Complete help.** Every command has `Short`, `Long` and `Example`, written
  for someone who has never used limabean. A test walks the command tree and
  fails if any of them is empty.
- **Flags bound through Viper**, so each setting can come from a flag, a
  `LIMABEAN_*` environment variable or the config file, in that order.
- **No secret values in argv.** Values reach `security`, `ssh` and other
  processes on stdin.

## Configuration

Viper reads `~/.config/limabean/config`, the existing `KEY="value"` file, using
its `dotenv` format, so configs written by the bash `init` keep working.

| Key | Default | Flag | Environment |
|---|---|---|---|
| `GIT_CONFIG_DIR` | `~/.config/git` | | `LIMABEAN_GIT_CONFIG_DIR` |
| `CODE_DIR` | `~/code` | | `LIMABEAN_CODE_DIR` |
| `CODE_DISK` | `50` (GiB) | | `LIMABEAN_CODE_DISK` |
| `CPUS` / `MEMORY` / `DISK` | template defaults (2, 4 GiB, 10 GiB) | `--cpus` / `--memory` / `--disk` on `new` | `LIMABEAN_CPUS` / `LIMABEAN_MEMORY` / `LIMABEAN_DISK` |
| `PACKS` | `go,python,node,kubernetes,infra,containers,db` | `--with` / `--without` on `new` | `LIMABEAN_PACKS` |
| `IMAGES` | `yes` | | `LIMABEAN_IMAGES` |
| `CLAUDE_DESKTOP` | `yes` | | `LIMABEAN_CLAUDE_DESKTOP` |
| `AWS_DIR` | `~/.aws` | | `LIMABEAN_AWS_DIR` |
| `IDE` | empty: the first installed | `--ide` on `code` | `LIMABEAN_IDE` |

Global flags: `--config`, `--plain` (also `LIMABEAN_PLAIN=1`), `--templates`
(use a templates directory instead of the embedded copy, for development).

## Templates

`templates/` is embedded with `go:embed`. Lima needs real files, and it
resolves `file:` and `base:` references relative to the template. So on
first use the tree is written to `~/.cache/limabean/templates/<hash>/`, named
by a hash of its contents, and `limactl start` reads it from there. Your own
environments in `~/.config/limabean/templates/*.yaml` are copied alongside
it, so they can use the built-in layers. `--templates ./templates` points at
a checkout instead.

## Interface

- In a terminal: lipgloss styles in the README palette, a spinner for slow
  steps (Lima's output is shown only on failure), huh forms and selects for
  prompts, and a table for `ls`.
- Without a terminal, or with `--plain`: plain lines, no prompts. Missing
  arguments are errors, and questions take their defaults.

## Secrets

References live on the Mac, never values:

```
~/.config/limabean/secrets.env         every VM
~/.config/limabean/secrets.<vm>.env    one VM; overrides the shared file
```

One `NAME=reference` per line. A reference is `keychain:<service>`,
`op://vault/item/field` (1Password CLI), `env:<VAR>`, `cmd:<command>`, or a
plain value for non-secret settings. `limabean secrets sync` resolves them on
the Mac and writes `export NAME='value'` lines to
`~/.config/limabean/secrets.env` (mode 600) in each VM. The VM's `env.sh`
loads that file.

Commands: `secrets ls`, `secrets set`, `secrets rm`, `secrets edit` and
`secrets sync`. `secrets set NAME`, given no reference, prompts for the value
and stores it in the Keychain as `limabean-secret-NAME`.

**At creation**, `limabean new` shows a form: the name, the environment, the
existing secrets to include (all shared ones are pre-selected), and "Add a
secret…", which asks for a name and a source (paste into the Keychain, a
1Password reference, an environment variable or a command). Choices made here
are written to `secrets.<vm>.env`. For scripts, `--secret NAME=reference`
does the same.

**GitHub token.** `limabean github login` stores the token in the Keychain
(`limabean-github-token`) and sets `GITHUB_TOKEN=keychain:limabean-github-token`
in the shared file, so the token is just another secret. `github status`,
`sync` and `logout` remain as shortcuts.

## Testing

- Unit tests for reference parsing and merging, shell quoting of values,
  Claude settings edits (other keys kept, invalid JSON untouched, in-place
  writes), config loading, and `limactl list` JSON parsing through a fake
  runner.
- The help-completeness test described under Conventions.
- CI runs `go test ./...`, `go vet` and golangci-lint, alongside the existing
  shellcheck and template validation.
- End-to-end runs against real VMs before each release.

## Migration

Done. The Go CLI reads the same config, writes the same Claude Desktop
entries and Keychain items, and creates VMs from the same templates, so
existing setups keep working. The bash `bin/limabean` is gone. VMs created by
it load secrets after `limabean rebuild`.

## Releases

GoReleaser ([`.goreleaser.yaml`](https://github.com/rossedman/limabean/blob/main/.goreleaser.yaml)) builds one universal
macOS binary, with the version set through `-X main.version`. Pull requests
get snapshot builds as workflow artifacts (`snapshot.yml`). A `v*` tag runs
`release.yml`, which tests, builds and publishes a GitHub release with the
archive, checksums and a changelog, using the workflow's own `GITHUB_TOKEN`.
No other secrets are needed.
