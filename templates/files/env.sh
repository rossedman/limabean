# Environment for every shell, interactive or not, so `ssh lima-<name> <cmd>`
# sees the same tools as a login. Sourced by ~/.zshenv and the top of
# ~/.bashrc. Written by layer-base.yaml; edit templates/files/env.sh.
export PATH="$HOME/.local/bin:$HOME/.local/share/mise/shims:$PATH"
export EDITOR=vim
# kind looks for docker by default; `docker` here is nerdctl.
export KIND_EXPERIMENTAL_PROVIDER=nerdctl
