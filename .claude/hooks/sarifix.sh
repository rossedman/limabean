#!/bin/bash
# Stop hook: before Claude finishes, report the scanner findings its changes
# introduced (sarifix.yaml) so it fixes them. sarifix always exits 0 here and
# answers in JSON. Skipped when sarifix isn't installed.
set -euo pipefail

command -v sarifix >/dev/null || exit 0
exec sarifix check --hook --log-level warn
