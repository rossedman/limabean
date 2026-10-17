#!/bin/bash
# Generate zsh completions for the installed tools into one fpath directory
# (~/.zshrc adds it). Generated at boot rather than on every shell start, so
# shells start fast, and only when the tools changed, so boots are fast too.
# Runs after mise.sh, on every boot.
set -eux -o pipefail

# shellcheck source=/dev/null
. ~/.config/limabean/env.sh
dir=~/.local/share/zsh/completions
mkdir -p "$dir"

# The tools are the packs' pinned versions, Docker and pip: unchanged since
# the last run (always, in a VM copied from an image), there's nothing to do.
stamp=$({ cat /etc/limabean/mise/*.toml; docker --version; pip3 --version; } 2>/dev/null | sha256sum | cut -d' ' -f1)
if [ "$(cat "$dir/.stamp" 2>/dev/null)" = "$stamp" ]; then
  exit 0
fi

# gen <name> <command…>: write _<name> from the command's output, starting at
# its #compdef line (zsh ignores the file unless that's line 1). Skip the tool,
# without failing provisioning, if the command fails or prints no #compdef.
gen() {
  local name=$1
  shift
  if "$@" 2>/dev/null | sed -n '/^#compdef/,$p' > "$dir/_$name.tmp" && [ -s "$dir/_$name.tmp" ]; then
    mv "$dir/_$name.tmp" "$dir/_$name"
  else
    rm -f "$dir/_$name.tmp"
    echo >&2 "limabean: no zsh completion for $name"
  fi
}

gen gh            gh completion -s zsh
gen kubectl       kubectl completion zsh
gen helm          helm completion zsh
gen kind          kind completion zsh
gen k9s           k9s completion zsh
gen docker        docker completion zsh
gen mise          mise completion zsh
gen goreleaser    goreleaser completion zsh
gen golangci-lint golangci-lint completion zsh
gen uv            uv generate-shell-completion zsh
gen uvx           uvx --generate-shell-completion zsh
gen pip           pip3 completion --zsh
gen cosign        cosign completion zsh
gen syft          syft completion zsh
gen grype         grype completion zsh
gen glow          glow completion zsh
gen gum           gum completion zsh
gen svu           svu completion zsh
gen rg            rg --generate complete-zsh
gen bat           bat --completion zsh
gen yq            yq shell-completion zsh
gen fd            fd --gen-completions zsh

# ghq ships its completion file instead of generating one.
ghq_comp=$(find "$(mise where ghq)" -path '*/zsh/_ghq' | head -1)
[ -n "$ghq_comp" ] && cp "$ghq_comp" "$dir/_ghq"

# Not generated here:
# - aws, terraform and go (gocomplete) use bash-style completion, wired up
#   in zshrc.
# - git, go, ssh, make, tmux, systemctl and others ship with zsh.

# Force zsh to rebuild its completion cache on the next shell start.
rm -f ~/.zcompdump*
echo "$stamp" > "$dir/.stamp"
