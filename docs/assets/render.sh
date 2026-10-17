#!/bin/bash
# Render the README graphics in docs/assets/src/*.html to PNGs in docs/assets/,
# at 2x for sharp text. Needs Google Chrome (fonts load from Google Fonts).
# Usage: docs/assets/render.sh [name…]   (default: all)
set -euo pipefail

cd "$(dirname "$0")"
chrome=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}

# name width height (CSS pixels; the PNG is twice that)
targets="
banner 1280 400
how-it-works 1280 660
terminal 1280 760
social-preview 1280 640
"

echo "$targets" | while read -r name w h; do
  [ -n "$name" ] || continue
  if [ $# -gt 0 ] && ! printf '%s\n' "$@" | grep -qx "$name"; then continue; fi
  "$chrome" --headless=new --disable-gpu --hide-scrollbars --no-first-run \
    --force-device-scale-factor=2 --window-size="$w,$h" \
    --virtual-time-budget=8000 --run-all-compositor-stages-before-draw \
    --screenshot="$PWD/$name.png" "file://$PWD/src/$name.html" 2>/dev/null
  echo "rendered $name.png"
done
