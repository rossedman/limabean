#!/bin/bash
# Claude Code in a limabean VM: its status line, what it should know about
# the VM, and no traffic a locked network would only block. Runs on every
# boot, for the claude pack's Claude Code and Claude Desktop's over SSH.
#
# Your own choices win: settings.json keys you've set are left alone, and
# CLAUDE.md is yours apart from the block between limabean's markers.
set -eux -o pipefail

claude=$HOME/.claude
settings=$claude/settings.json
memory=$claude/CLAUDE.md
mkdir -p "$claude"

# settings.json: limabean's status line unless there's another one, and no
# nonessential traffic (update checks, error reports) unless set otherwise.
[ -s "$settings" ] || echo '{}' > "$settings"
if jq -e 'type == "object"' "$settings" >/dev/null 2>&1; then
  jq '.statusLine //= {"type": "command", "command": "limabean-statusline", "padding": 0}
      | .env //= {}
      | .env.CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC //= "1"' "$settings" > "$settings.new"
  mv "$settings.new" "$settings"
else
  echo >&2 "limabean: $settings isn't a JSON object, so it was left as it is"
fi

# CLAUDE.md: limabean's block, replaced on every boot so it stays current.
begin='<!-- limabean: this block is rewritten at boot (templates/features/claude.sh); write your own notes outside it -->'
end='<!-- /limabean -->'
block=$(cat <<EOF
$begin
## This machine is a limabean VM

- You're in a Linux VM on the user's Mac, made by limabean. Run \`limabean-info\` to see what it has: its tools, the project's clone, which hosts the network allows, and its secrets by name. Check there before guessing.
- The network may only reach an allowlist. When a download or request can't resolve or connect, the host is probably blocked: stop retrying, name the host, and tell the user they can allow it on their Mac with \`limabean network allow <vm> <host>\`. You can't change the allowlist, and you may not have sudo.
- A program listening on a port here is at http://localhost:<port> on the Mac.
- The repository here is a clone, separate from the user's checkout on the Mac. Commit your work on a branch: the user fetches it with \`limabean remote\`, or it's pushed.
- Secrets are environment variables. Never print their values.
$end
EOF
)
touch "$memory"
others=$(awk -v b="$begin" -v e="$end" '$0 == b { skip = 1; next } $0 == e { skip = 0; next } !skip' "$memory")
{
  printf '%s\n' "$block"
  [ -n "$others" ] && printf '\n%s\n' "$others"
  true
} > "$memory.new"
mv "$memory.new" "$memory"
