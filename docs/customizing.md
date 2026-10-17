# Customizing

Templates live in [`templates/`](../templates) and are layered with Lima's
`base:` composition:

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
| Add an environment | a new `templates/<env>.yaml` with `base: [./layer-base.yaml]`, then `limabean new <name> <env>` |

VMs are disposable, and rebuilding is how changes get applied:

```bash
limabean rebuild dev
```

Lima doesn't allow `..` in relative paths or base template file names starting
with `_`. That's why everything sits under `templates/` and the layers are
named `layer-*.yaml`.

## Checks

CI ([`lint.yml`](../.github/workflows/lint.yml)) runs `shellcheck` on
`bin/limabean` and `templates/features/*.sh`, and `limactl tmpl validate` on
each environment template. Run the same checks locally before opening a PR.

## README graphics

The images in [`docs/assets`](assets) are rendered from the HTML sources in
`docs/assets/src` with Google Chrome:

```bash
docs/assets/render.sh            # all images, or name one: render.sh banner
```

`social-preview.png` is the image GitHub shows when the repository is linked.
Upload it under the repository's **Settings → General → Social preview**.
