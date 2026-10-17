# Customizing

Templates live in [`templates/`](../templates) and are built into the
limabean binary, which writes them to `~/.cache/limabean/templates/` for Lima
to read. They're layered with Lima's `base:` composition:

```
templates/
  layer-os.yaml       Ubuntu 24.04, VM size, agent forwarding, zsh login shell
  layer-base.yaml     everything every VM gets (built on layer-os)
  scratch.yaml        an environment (built on layer-base)
  features/           idempotent provisioning scripts, one per capability
  files/              files written into the VM (zshrc, env.sh, editorconfig)
  mise/               pinned tool versions
```

| To… | Edit |
|---|---|
| Add or upgrade a CLI tool | [`mise/base.toml`](../templates/mise/base.toml). Pin a version; `mise latest <tool>` in a VM shows the newest. |
| Add a system package | `packages=(…)` in [`features/apt-common.sh`](../templates/features/apt-common.sh) |
| Change the prompt, aliases or shell options | [`files/zshrc`](../templates/files/zshrc) |
| Add an environment variable for every shell | [`files/env.sh`](../templates/files/env.sh) |
| Add a tool's zsh completion | a `gen` line in [`features/completions.sh`](../templates/features/completions.sh) |
| Change the global editorconfig | [`files/editorconfig`](../templates/files/editorconfig) |
| Change CPU or memory | `CPUS`/`MEMORY` in `~/.config/limabean/config`, or the defaults in [`layer-os.yaml`](../templates/layer-os.yaml) |
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

CI ([`lint.yml`](../.github/workflows/lint.yml)) runs gofmt, `go vet`,
`go test`, `shellcheck` on the provisioning scripts, `limactl tmpl validate`
on each environment template, and `goreleaser check`. Locally:

```bash
go test ./... && go vet ./...
LIMABEAN_TEST_KEYCHAIN=1 go test ./internal/keychain/   # also tests against your login Keychain
```

## Building and releasing

limabean is built with [GoReleaser](https://goreleaser.com)
([`.goreleaser.yaml`](../.goreleaser.yaml)) as one universal macOS binary:

```bash
goreleaser release --snapshot --clean   # → dist/limabean_darwin_all/limabean
```

- Every pull request gets a snapshot build
  ([`snapshot.yml`](../.github/workflows/snapshot.yml)), downloadable from the
  run's **Artifacts**.
- Pushing a tag such as `v0.1.0` runs [`release.yml`](../.github/workflows/release.yml),
  which tests, builds and publishes a GitHub release with the archive,
  checksums and a changelog.

## README graphics

The images in [`docs/assets`](assets) are rendered from the HTML sources in
`docs/assets/src` with Google Chrome:

```bash
docs/assets/render.sh            # all images, or name one: render.sh banner
```

`social-preview.png` is the image GitHub shows when the repository is linked.
Upload it under the repository's **Settings → General → Social preview**.
