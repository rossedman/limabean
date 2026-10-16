#!/bin/bash
# System packages shared by every environment. Runs as root on every boot.
set -eux -o pipefail

export DEBIAN_FRONTEND=noninteractive
packages=(git curl ca-certificates build-essential unzip xz-utils jq)

missing=()
for p in "${packages[@]}"; do
  dpkg -s "$p" >/dev/null 2>&1 || missing+=("$p")
done
if [ ${#missing[@]} -gt 0 ]; then
  apt-get update
  apt-get install -y "${missing[@]}"
fi
