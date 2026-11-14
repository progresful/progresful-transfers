#!/usr/bin/env bash
# Plays the team's five ranking runs of the current round at each given level (default 1-8),
# the levels side by side, one run at a time per level as the server requires. Every run is
# logged like any other (label "board"). A level stops at its first failed run: an
# unfinished ranking run blocks the next one, and deleting it would count it as played.
#
#   scripts/board_round.sh            # levels 1-8
#   scripts/board_round.sh 8          # level 8 only
set -uo pipefail
cd "$(dirname "$0")/.."
levels=("$@")
[ ${#levels[@]} -eq 0 ] && levels=(1 2 3 4 5 6 7 8)
mkdir -p runs
stamp=$(date +%Y%m%d-%H%M)
for level in "${levels[@]}"; do
  (
    for week in 1 2 3 4 5; do
      out=$(./bin/hc run -level "$level" -board -label board 2>&1)
      status=$?
      echo "L${level} week ${week}: $(echo "$out" | grep -E '^End ' | sed 's/^End *//')$(echo "$out" | grep -E '^hc:' )"
      [ $status -ne 0 ] && { echo "L${level}: stopped after a failed run"; break; }
    done
  ) | tee -a "runs/board-${stamp}.log" &
done
wait
echo "done: runs/board-${stamp}.log"
