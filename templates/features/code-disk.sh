#!/bin/bash
# Persistent code directory: when limabean attached a disk for it (the
# codeDisk param), mount it over the codeDir param, so repositories survive
# `limabean rebuild`. Lima formats the disk on first use and mounts it at
# /mnt/lima-<disk> before provisioning; this bind-mounts it into place.
# Runs as root on every boot, before the user scripts (git.sh) use codeDir.
set -eux -o pipefail

# shellcheck disable=SC2154  # PARAM_* and LIMA_CIDATA_* are set by Lima
disk=${PARAM_codeDisk:-}
[ -n "$disk" ] || exit 0

src=/mnt/lima-$disk
if ! mountpoint -q "$src"; then
  echo "limabean: $src isn't mounted; ~/code stays on the VM's disk" >&2
  exit 1
fi
# shellcheck disable=SC2154
user=$LIMA_CIDATA_USER
# shellcheck disable=SC2154
code_dir="${PARAM_codeDir/#\~/$LIMA_CIDATA_HOME}"
group=$(id -gn "$user")

chown "$user:$group" "$src"
sudo -u "$user" mkdir -p "$code_dir"
mountpoint -q "$code_dir" || mount --bind "$src" "$code_dir"
