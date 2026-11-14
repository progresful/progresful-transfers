#!/usr/bin/env bash
# Saves a screenshot of the public leaderboard page to screens/ (ignored by git).
# The page keeps a live connection open, so the capture is forced after a fixed time,
# and a throwaway browser profile keeps it away from the normal Chrome profile.
#
#   scripts/screenshot_leaderboard.sh
set -euo pipefail
cd "$(dirname "$0")/.."
chrome="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
[ -x "$chrome" ] || { echo "Chrome not found at $chrome (set CHROME=...)" >&2; exit 1; }
mkdir -p screens
profile=$(mktemp -d)
trap 'rm -rf "$profile"' EXIT
out="screens/leaderboard-$(date +%Y%m%d-%H%M).png"
"$chrome" --headless=new --user-data-dir="$profile" --no-first-run --disable-gpu --hide-scrollbars \
  --window-size=1500,2400 --timeout=15000 --screenshot="$PWD/$out" \
  "https://spread.bahnexpress.com/leaderboard" >/dev/null 2>&1 &
pid=$!
( sleep 50; kill "$pid" 2>/dev/null ) &
wait "$pid" 2>/dev/null || true
[ -s "$out" ] && echo "saved $out" || { echo "no screenshot was written" >&2; exit 1; }
