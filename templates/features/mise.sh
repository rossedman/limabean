#!/bin/bash
# Install mise and the tools listed in /etc/limabean/mise/*.toml (PATH is set
# by files/env.sh). Runs as the VM user on every boot.
set -eux -o pipefail

if [ ! -x ~/.local/bin/mise ]; then
  curl -fsSL https://mise.run | sh
fi

# Link the environment's TOML files into mise's global config directory.
mkdir -p ~/.config/mise/conf.d
for f in /etc/limabean/mise/*.toml; do
  ln -sf "$f" ~/.config/mise/conf.d/
done

~/.local/bin/mise install --yes

# Tell the probe in layer-base.yaml that provisioning finished.
mkdir -p ~/.local/state/limabean
touch ~/.local/state/limabean/mise.done
