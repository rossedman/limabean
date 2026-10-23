#!/bin/sh
# A sarifix scanner: shellcheck has no SARIF output, so run it with -f json1
# over the same files as CLAUDE.md and lint.yml, and convert with jq.
# Usage: shellcheck.sh DIR OUT
set -eu
cd "$1"
out=$2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Exit 1 means findings; anything higher (a missing file, bad flags) fails the scan.
lint() {
  rc=0
  shellcheck -f json1 "$@" || rc=$?
  [ "$rc" -le 1 ]
}
lint templates/features/*.sh templates/files/limabean-network >"$tmp/bash.json"
lint -s sh templates/files/env.sh templates/files/limabean-github-git \
  templates/files/limabean-info templates/files/limabean-statusline >"$tmp/sh.json"

jq -s '[.[].comments[]] as $c | {
  version: "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  runs: [{
    tool: {driver: {
      name: "shellcheck",
      informationUri: "https://www.shellcheck.net",
      rules: [[$c[].code] | unique | .[] | {id: "SC\(.)", helpUri: "https://www.shellcheck.net/wiki/SC\(.)"}]
    }},
    results: [$c[] | {
      ruleId: "SC\(.code)",
      level: ({error: "error", warning: "warning"}[.level] // "note"),
      message: {text: .message},
      locations: [{physicalLocation: {
        artifactLocation: {uri: .file},
        region: {startLine: .line, startColumn: .column, endLine: .endLine, endColumn: .endColumn}
      }}]
    }]
  }]
}' "$tmp/bash.json" "$tmp/sh.json" >"$out"
