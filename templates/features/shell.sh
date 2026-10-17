#!/bin/bash
# bash setup, for anything that runs bash explicitly. zsh is the login shell
# (user.shell in layer-os.yaml; config files written by layer-base.yaml), and
# bash shares the same environment. Runs on every boot.
set -eux -o pipefail

# bash: load env.sh before Ubuntu's non-interactive guard, add hooks after it.
marker='# limabean: env'
if ! grep -qF "$marker" ~/.bashrc; then
  sed -i "1i . \"\$HOME/.config/limabean/env.sh\" $marker" ~/.bashrc
  cat >> ~/.bashrc <<'HOOKS'

# limabean: hooks (templates/features/shell.sh)
command -v direnv >/dev/null && eval "$(direnv hook bash)"
HOOKS
fi
