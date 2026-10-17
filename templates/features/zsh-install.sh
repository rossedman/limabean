#!/bin/bash
# Install zsh before anyone logs in: the VM user's login shell is
# /usr/bin/zsh (user.shell in layer-os.yaml), and SSH logins fail until it
# exists. Dependency mode runs during Lima's boot scripts, ahead of
# provisioning.
set -eux -o pipefail

if ! command -v zsh >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y zsh
fi
