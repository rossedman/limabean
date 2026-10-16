#!/bin/bash
# Install mise, activate it for every shell, and install the tools listed in
# /etc/limabean/mise/*.toml. Runs as the VM user on every boot.
set -eux -o pipefail

if [ ! -x ~/.local/bin/mise ]; then
  curl -fsSL https://mise.run | sh
fi

# Link the environment's TOML files into mise's global config directory.
mkdir -p ~/.config/mise/conf.d
for f in /etc/limabean/mise/*.toml; do
  ln -sf "$f" ~/.config/mise/conf.d/
done

# Put mise shims on PATH before Ubuntu's non-interactive guard in .bashrc, so
# `ssh lima-<name> <cmd>` sees the tools too.
marker='# limabean: PATH'
if ! grep -qF "$marker" ~/.bashrc; then
  line="export PATH=\"\$HOME/.local/bin:\$HOME/.local/share/mise/shims:\$PATH\" $marker"
  sed -i "1i $line" ~/.bashrc
fi

~/.local/bin/mise install --yes

# Tell the probe in layer-base.yaml that provisioning finished.
mkdir -p ~/.local/state/limabean
touch ~/.local/state/limabean/mise.done
