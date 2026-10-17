#!/bin/bash
# Git setup: use the host's git config (mounted read-only at /mnt/host/git
# from the gitConfigDir param), set up ghq to clone into the codeDir param, and
# trust GitHub's SSH host keys. Pushing uses
# the host's ssh-agent via agent forwarding, so private keys never enter the
# VM. Runs on every boot.
set -eux -o pipefail

host_git=/mnt/host/git
# shellcheck disable=SC2154  # PARAM_codeDir is set by Lima from the codeDir param
code_dir="${PARAM_codeDir/#\~/$HOME}"
mkdir -p "$code_dir"

# First file that exists, out of the names git uses for each role.
first() { for f in "$@"; do [ -f "$host_git/$f" ] && { echo "$host_git/$f"; return; }; done; }
config=$(first config .gitconfig)
ignore=$(first ignore .gitignore)
template=$(first .gitcommit.txt commit-template.txt)

# Rewritten on every boot; edit this script, not ~/.gitconfig in the VM.
# Order: limabean defaults, then the host config (which can override them),
# then values that only make sense on the host and must be replaced.
{
  echo "# Managed by limabean (templates/features/git.sh)."
  # ghq uses the first ghq.root as its primary root, so this one wins over a
  # host-specific root; the host's per-URL [ghq "<url>"] roots still apply.
  printf '[ghq]\n  root = %s\n' "$code_dir"
  # ghq clones GitHub over https, which needs no credentials for public repos;
  # push over SSH so the forwarded agent authenticates it.
  printf '[url "git@github.com:"]\n  pushInsteadOf = https://github.com/\n'
  [ -n "$config" ] && printf '[include]\n  path = %s\n' "$config"
  printf '[core]\n  editor = vim\n'
  [ -n "$ignore" ] && printf '  excludesfile = %s\n' "$ignore"
  [ -n "$template" ] && printf '[commit]\n  template = %s\n' "$template"
  true
} > ~/.gitconfig

if [ -z "$config" ]; then
  echo >&2 "limabean: no git config found in the gitConfigDir mount; see README.md"
fi

# GitHub host keys from its TLS-verified API, so the first push doesn't stop
# at a host key prompt.
mkdir -p ~/.ssh && chmod 700 ~/.ssh
if ! grep -q '^github.com ' ~/.ssh/known_hosts 2>/dev/null; then
  curl -fsSL https://api.github.com/meta | jq -r '.ssh_keys[] | "github.com " + .' >> ~/.ssh/known_hosts
fi
