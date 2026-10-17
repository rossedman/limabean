# Environment for every shell, interactive or not, so `ssh lima-<name> <cmd>`
# sees the same tools as a login. Sourced by ~/.zshenv and the top of
# ~/.bashrc. Written by layer-base.yaml; edit templates/files/env.sh.

# Go: `go install` puts binaries in $GOBIN.
export GOPATH="$HOME/go"
export GOBIN="$GOPATH/bin"

# Python: `pip install --user`, `uv tool install` and pipx put scripts in
# $PYTHONUSERBASE/bin, which is ~/.local/bin (also home to mise and the docker
# alias).
export PYTHONUSERBASE="$HOME/.local"

# Resulting order: $GOBIN, ~/.local/bin, mise tools, then the system.
# Each directory is added once, so nested shells don't grow PATH.
for d in "$HOME/.local/share/mise/shims" "$PYTHONUSERBASE/bin" "$GOBIN"; do
  case ":$PATH:" in
    *":$d:"*) ;;
    *) PATH="$d:$PATH" ;;
  esac
done
export PATH

export EDITOR=vim
# bat (also git's pager): use the terminal's own 16-color palette instead of a
# fixed theme, so highlighting matches your terminal's color scheme.
export BAT_THEME=ansi
# kind looks for docker by default; `docker` here is nerdctl.
export KIND_EXPERIMENTAL_PROVIDER=nerdctl
