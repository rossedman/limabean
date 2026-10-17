# Customizing

Templates live in [`templates/`](https://github.com/rossedman/limabean/tree/main/templates) and are built into the
limabean binary, which writes them to `~/.cache/limabean/templates/` for Lima
to read. They're layered with Lima's `base:` composition:

```
templates/
  layer-os.yaml       Ubuntu 26.04 LTS, VM size, agent forwarding, zsh login shell
  layer-base.yaml     everything every VM gets (built on layer-os)
  scratch.yaml        an environment (built on layer-base)
  features/           idempotent provisioning scripts, one per capability
  files/              files written into the VM (zshrc, env.sh, editorconfig)
  mise/               pinned tool versions
```

| To… | Edit |
|---|---|
| Add or upgrade a CLI tool | [`mise/base.toml`](https://github.com/rossedman/limabean/blob/main/templates/mise/base.toml). Pin a version; `limabean bump` upgrades every pin within its major (`--major` for more), and a monthly workflow (`bump.yml`) opens a PR with the same. |
| Add a system package | `packages=(…)` in [`features/apt-common.sh`](https://github.com/rossedman/limabean/blob/main/templates/features/apt-common.sh) |
| Change the prompt, aliases or shell options | [`files/zshrc`](https://github.com/rossedman/limabean/blob/main/templates/files/zshrc) |
| Add an environment variable for every shell | [`files/env.sh`](https://github.com/rossedman/limabean/blob/main/templates/files/env.sh) |
| Add a tool's zsh completion | a `gen` line in [`features/completions.sh`](https://github.com/rossedman/limabean/blob/main/templates/features/completions.sh) |
| Change the global editorconfig | [`files/editorconfig`](https://github.com/rossedman/limabean/blob/main/templates/files/editorconfig) |
| Change CPU or memory | `CPUS`/`MEMORY` in `~/.config/limabean/config`, or the defaults in [`layer-os.yaml`](https://github.com/rossedman/limabean/blob/main/templates/layer-os.yaml) |
| Add your own environment | `~/.config/limabean/templates/<env>.yaml` with `base: [./layer-base.yaml]`, then `limabean new <name> <env>`. No rebuild of limabean needed. |

VMs are disposable, and rebuilding is how changes get applied:

```bash
limabean rebuild dev
```

Lima doesn't allow `..` in relative paths or base template file names starting
with `_`. That's why everything sits under `templates/` and the layers are
named `layer-*.yaml`.

## Working on the templates

Changes to `templates/` reach a VM after you rebuild limabean, or straight
away if you point limabean at your checkout:

```bash
limabean --templates ./templates rebuild dev
```

## Checks

CI ([`lint.yml`](https://github.com/rossedman/limabean/blob/main/.github/workflows/lint.yml)) runs gofmt, `go vet`,
`go test` (failing if total coverage drops below 90%), `shellcheck` on the
provisioning scripts, `limactl tmpl validate` on each environment template,
and `goreleaser check`. Locally:

```bash
go test ./... && go vet ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
LIMABEAN_TEST_KEYCHAIN=1 go test ./internal/keychain/   # also tests against your login Keychain
```

Command tests run every command against a fake Lima, Keychain and GitHub API.
Forms are driven with keystrokes: `ui.NewTerminalForms` gives each form its
own `ui.Keys(…)` input.

## Building and releasing

limabean is built with [GoReleaser](https://goreleaser.com)
([`.goreleaser.yaml`](https://github.com/rossedman/limabean/blob/main/.goreleaser.yaml)) as one universal macOS binary:

```bash
goreleaser release --snapshot --clean   # → dist/limabean_darwin_all/limabean
```

- Every pull request gets a snapshot build
  ([`snapshot.yml`](https://github.com/rossedman/limabean/blob/main/.github/workflows/snapshot.yml)), downloadable from the
  run's **Artifacts**.
- Pushing a tag such as `v0.1.0` runs [`release.yml`](https://github.com/rossedman/limabean/blob/main/.github/workflows/release.yml),
  which tests, builds and publishes a GitHub release with the archive,
  checksums and a changelog.

## README and site graphics

The images in [`docs/assets`](https://github.com/rossedman/limabean/tree/main/docs/assets) are rendered from the HTML sources in
`docs/assets/src` with Google Chrome:

```bash
docs/assets/render.sh            # all images, or name one: render.sh banner
```

`social-preview.png` is the image GitHub shows when the repository is linked.
Upload it under the repository's **Settings → General → Social preview**.
