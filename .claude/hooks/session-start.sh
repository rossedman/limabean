#!/bin/bash
# SessionStart hook for Claude Code cloud sessions: install what the checks
# in CLAUDE.md need (Go modules, shellcheck, actionlint, MkDocs). Versions
# match CI (.github/workflows/lint.yml and pages.yml). Does nothing on a
# Mac, where you install your own tools.
set -euo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi
cd "${CLAUDE_PROJECT_DIR:-.}"

# Go modules, and the toolchain go.mod asks for.
go mod download

# The shell linter, for templates/features/*.sh.
if ! command -v shellcheck >/dev/null; then
  sudo=""
  [ "$(id -u)" -eq 0 ] || sudo="sudo -n"
  $sudo apt-get update -qq
  $sudo apt-get install -y -qq shellcheck
fi

# actionlint, for .github/workflows.
gobin=$(go env GOPATH)/bin
if ! command -v actionlint >/dev/null && [ ! -x "$gobin/actionlint" ]; then
  go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.7
fi

# sarifix and the scanners in sarifix.yaml, for the Stop hook. Pinned: a
# scanner upgrade can report old findings as new (then: sarifix check --refresh).
if [ ! -x "$gobin/sarifix" ]; then
  go install github.com/rossedman/sarifix/cmd/sarifix@v0.2.1
fi
if [ ! -x "$gobin/golangci-lint" ]; then
  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
fi
echo "export PATH=\"$gobin:\$PATH\"" >> "${CLAUDE_ENV_FILE:-/dev/null}"

# MkDocs 1.x with Material, as pages.yml builds the site.
if ! python3 -c 'import material' 2>/dev/null; then
  pip install --quiet "mkdocs>=1.6,<2" "mkdocs-material>=9.7,<10"
fi
