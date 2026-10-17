#!/bin/bash
# Network allowlist and lockdown (see files/limabean-network). Installs
# what limabean-network needs and applies the policy limabean wrote on the
# Mac, mounted read-only at /mnt/host/limabean. Runs as root on every boot,
# after docker.sh, so a locked VM is locked again before anyone logs in.
set -eux -o pipefail

export DEBIAN_FRONTEND=noninteractive
missing=()
for p in nftables dnsmasq-base; do
  dpkg -s "$p" >/dev/null 2>&1 || missing+=("$p")
done
if [ "${#missing[@]}" -gt 0 ]; then
  apt-get update
  apt-get install -y "${missing[@]}"
fi

# dnsmasq drops to this user; the firewall lets only it reach upstream DNS.
id limabean-dns >/dev/null 2>&1 ||
  useradd --system --no-create-home --shell /usr/sbin/nologin limabean-dns

mkdir -p /etc/limabean
echo "{{.User}}" > /etc/limabean/user

unit=/etc/systemd/system/limabean-network.service
want='[Unit]
Description=limabean network allowlist and lockdown
After=network-online.target docker.service
Wants=network-online.target
RequiresMountsFor=/mnt/host/limabean

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/limabean-network

[Install]
WantedBy=multi-user.target'
if [ "$(cat "$unit" 2>/dev/null)" != "$want" ]; then
  echo "$want" > "$unit"
  systemctl daemon-reload
fi
systemctl enable limabean-network.service

# On the first boot the toolchain is still being installed (user scripts run
# after this one), so limabean applies the policy once provisioning is done.
if [ ! -e "{{.Home}}/.local/state/limabean/mise.done" ]; then
  exit 0
fi
systemctl start limabean-network.service
