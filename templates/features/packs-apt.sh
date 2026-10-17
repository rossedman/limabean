#!/bin/bash
# System packages for the VM's packs: limabean writes each pack's apt list
# to /etc/limabean/apt/<pack>.list (one package per line). Runs as root on
# every boot, so packages added by a rebuild get installed.
set -eux -o pipefail

shopt -s nullglob
lists=(/etc/limabean/apt/*.list)
[ "${#lists[@]}" -gt 0 ] || exit 0

export DEBIAN_FRONTEND=noninteractive
missing=()
while read -r p; do
  [ -n "$p" ] || continue
  dpkg -s "$p" >/dev/null 2>&1 || missing+=("$p")
done < <(cat "${lists[@]}" | sed 's/#.*//' | tr -s ' \t' '\n' | sort -u)
if [ "${#missing[@]}" -gt 0 ]; then
  apt-get update
  apt-get install -y "${missing[@]}"
fi
