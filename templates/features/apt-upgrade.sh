#!/bin/bash
# Bring every package up to date, systemd included, when the VM is created.
# Only the first boot does this; `limabean rebuild` is how an existing VM gets
# newer packages. Runs as root.
set -eux -o pipefail

marker=/var/lib/limabean/upgraded
[ -f "$marker" ] && exit 0

export DEBIAN_FRONTEND=noninteractive
# needrestart would otherwise restart services mid-provisioning; anything that
# needs it gets a full restart from limabean instead (see reboot-required).
export NEEDRESTART_SUSPEND=1
# Wait for the dpkg lock instead of failing: cloud-init and unattended-upgrades
# may be using apt on first boot.
apt=(apt-get -o DPkg::Lock::Timeout=600
  -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold)

"${apt[@]}" update
"${apt[@]}" -y full-upgrade
"${apt[@]}" -y autoremove --purge

# Switch the running systemd (PID 1) to the upgraded version now.
systemctl daemon-reexec

mkdir -p "$(dirname "$marker")"
date -u +%FT%TZ > "$marker"
