---
paths:
  - "templates/**"
---

# Templates

- Layers: `layer-os.yaml` (Ubuntu, size, Rosetta, user) → `layer-base.yaml`
  (everything every VM gets) → environments such as `scratch.yaml`. Lima
  merges them into one `lima.yaml` when a VM is created.
- Lima rejects `..` in relative paths and base templates whose names start
  with `_`; keep everything under `templates/`, layers named `layer-*.yaml`.
- `features/*.sh` provisioning scripts run on **every boot**, also in VMs
  copied from an image, so they must be idempotent and fast when there's
  nothing to do: check before installing (`dpkg -s`, `command -v`, a stamp
  file), never assume a first boot.
- Scripts start with `set -eux -o pipefail` and must pass `shellcheck`
  (`env.sh`, `limabean-github-git`, `limabean-info` and `limabean-statusline` as `sh`). Lima's `PARAM_*` and
  `LIMA_CIDATA_*` variables need `# shellcheck disable=SC2154`.
- `limabean-network` runs as root and changes the VM's firewall and DNS; its
  allowlist mode can't be exercised in a container (it would firewall the
  container). Test learn and open modes with the `LIMABEAN_NETWORK_*`
  overrides into a scratch directory.
- Packs are `packs/<name>.toml` (first line a `# description`, pinned
  `[tools]`) plus an optional `<name>.apt`. Pin exact versions; update them
  with `limabean bump`, not by hand.
- Any change under `templates/` changes the templates hash, so every image
  becomes outdated and the next VM of each kind is provisioned from scratch.
  That's intended; mention it in the PR when it matters.
- Template changes are validated by `limactl tmpl validate` on macOS CI only.
