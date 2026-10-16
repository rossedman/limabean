#!/bin/bash
# Alias `docker` to nerdctl (installed by Lima, see `containerd:` in
# layer-os.yaml). A symlink rather than a shell alias, so scripts and
# non-interactive SSH commands get it too. Runs on every boot.
set -eux -o pipefail

mkdir -p ~/.local/bin
ln -sf "$(command -v nerdctl)" ~/.local/bin/docker
