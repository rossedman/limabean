#!/bin/bash
# System packages shared by every environment. Runs as root on every boot.
set -eux -o pipefail

export DEBIAN_FRONTEND=noninteractive
packages=(
  git curl wget ca-certificates build-essential unzip xz-utils jq
  vim httpie tree htop tmux stow
  zsh zsh-autosuggestions zsh-syntax-highlighting
  git-extras git-quick-stats speedtest-cli
  python3 python3-pip python3-venv
  skopeo
)

missing=()
for p in "${packages[@]}"; do
  dpkg -s "$p" >/dev/null 2>&1 || missing+=("$p")
done
if [ ${#missing[@]} -gt 0 ]; then
  apt-get update
  apt-get install -y "${missing[@]}"
fi
