#!/bin/bash
# Turn on the guardrails Claude Code plugin (github.com/rossedman/guardrails)
# for every Claude Code that runs in the VM: the claude pack's, and the copy
# Claude Desktop installs over SSH. Both read ~/.claude/settings.json, and
# Claude Code clones a marketplace declared there, then fetches the plugins
# enabled there, when a session starts. The plugin downloads its own binary
# and uses the formatters and linters the packs put on PATH.
# Runs on every boot: entries already there, including guardrails turned off
# with false, are left as they are.
set -eux -o pipefail

mkdir -p ~/.claude
python3 - ~/.claude/settings.json <<'EOF'
import json, os, sys

path = sys.argv[1]
try:
    with open(path) as f:
        settings = json.load(f)
except FileNotFoundError:
    settings = {}
except ValueError:
    print("limabean: " + path + " isn't valid JSON; guardrails not turned on", file=sys.stderr)
    sys.exit(0)

before = json.dumps(settings, sort_keys=True)
settings.setdefault("extraKnownMarketplaces", {}).setdefault(
    "guardrails", {"source": {"source": "github", "repo": "rossedman/guardrails"}})
settings.setdefault("enabledPlugins", {}).setdefault("guardrails@guardrails", True)
if json.dumps(settings, sort_keys=True) == before:
    sys.exit(0)

tmp = path + ".tmp"
with open(tmp, "w") as f:
    json.dump(settings, f, indent=2)
    f.write("\n")
os.replace(tmp, path)
EOF
