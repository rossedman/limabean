#!/bin/bash
# Shell setup: zsh as the login shell (config files are written by
# layer-base.yaml), with bash still sharing the same environment. Runs on
# every boot.
set -eux -o pipefail

if [ "$(getent passwd "$USER" | cut -d: -f7)" != /usr/bin/zsh ]; then
  sudo usermod -s /usr/bin/zsh "$USER"
fi

# bash: load env.sh before Ubuntu's non-interactive guard, add hooks after it.
marker='# limabean: env'
if ! grep -qF "$marker" ~/.bashrc; then
  sed -i "1i . \"\$HOME/.config/limabean/env.sh\" $marker" ~/.bashrc
  cat >> ~/.bashrc <<'HOOKS'

# limabean: hooks (templates/features/shell.sh)
command -v direnv >/dev/null && eval "$(direnv hook bash)"
HOOKS
fi
