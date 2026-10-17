#!/bin/bash
# Docker Engine from Docker's official apt repository: dockerd, the docker CLI,
# and the buildx and compose plugins. BuildKit is the default builder, so
# Dockerfile frontends (`# syntax=docker/dockerfile:1`) work. Runs as root on
# every boot.
set -eux -o pipefail

if ! command -v docker >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  # shellcheck source=/dev/null
  . /etc/os-release
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi

systemctl enable --now docker.socket docker.service

# A locked VM's user doesn't get Docker: its socket is root-equivalent
# (features/network.sh, files/limabean-network).
if grep -qx locked /mnt/host/limabean/network.conf 2>/dev/null; then
  exit 0
fi

# Let the VM user run docker without sudo. Joining the docker group isn't
# enough on first boot: Lima's SSH connection is already open and keeps the
# group list it saw at login. So the user also owns the socket.
user="{{.User}}"
usermod -aG docker "$user"
conf=/etc/systemd/system/docker.socket.d/limabean.conf
want=$(printf '[Socket]\nSocketUser=%s' "$user")
if [ "$(cat "$conf" 2>/dev/null)" != "$want" ]; then
  mkdir -p "$(dirname "$conf")"
  echo "$want" > "$conf"
  systemctl daemon-reload
  systemctl restart docker.socket docker.service
fi
