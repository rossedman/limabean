#!/bin/bash
# PostToolUse hook: after Claude edits or writes a Go file, gofmt it, so
# formatting never fails CI. Reads the tool call's JSON on stdin.
set -euo pipefail

command -v jq >/dev/null && command -v gofmt >/dev/null || exit 0
file=$(jq -r '.tool_input.file_path // empty')
case "$file" in
  *.go) [ -f "$file" ] && gofmt -w "$file" ;;
esac
exit 0
