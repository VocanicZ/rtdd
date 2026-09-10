#!/usr/bin/env bash
# The agent-loop measurement: what RTDD costs against running everything, in the loop
# RTDD exists for.
#
# The replay corpus in bench/ replays COMMITTED commits, one per fresh checkout. That is
# the right shape for measuring selection quality and it is the wrong shape for measuring
# what an agent pays: an agent edits, tests, edits again, dozens of times, and commits
# once at the end. This script measures that loop directly, against the baseline the TDD
# skill prescribes — run the whole suite after every change.
#
# It is deliberately small and deliberately committed. A number nobody can re-run is not
# a number, and the figures in README.md's time axis come from here.
#
#   scripts/agent-session-bench.sh <clone-dir> <cycles>
#
# The clone must already have a working virtualenv on PATH and a seeded .rtdd (see
# DEVELOPMENT.md). Three sessions run in sequence over three copies of it, each applying
# the IDENTICAL edits to the same module: the full suite, `rtdd run` at its shipped
# default, and `rtdd run --record=auto`.
#
# What it does NOT measure: whether the selection caught what a full run would have. The
# edits are additive no-ops, so nothing fails in any arm — this is a cost measurement and
# only a cost measurement. Recall is bench/'s question and remains unanswered while the
# corpus contains no detecting commits.
set -euo pipefail

SRC=${1:?usage: agent-session-bench.sh <seeded-clone-dir> [cycles]}
CYCLES=${2:-10}
TARGET=${RTDD_BENCH_TARGET:-src/flask/views.py}
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

run_session() {
  local dir=$1 mode=$2 total=0
  cd "$dir"
  export PATH="$dir/.venv/bin:$PATH"
  for i in $(seq 1 "$CYCLES"); do
    printf '\n\ndef _agent_probe_%d():\n    return %d\n' "$i" "$i" >> "$TARGET"
    local s e ms
    s=$(date +%s%N)
    if [ "$mode" = full ]; then
      pytest -q -p no:cacheprovider >/dev/null 2>&1 || true
    else
      rtdd run --record="$mode" >/dev/null 2>&1 || true
    fi
    e=$(date +%s%N)
    ms=$(( (e - s) / 1000000 ))
    total=$(( total + ms ))
    printf '  cycle %2d  %6d ms\n' "$i" "$ms"
  done
  printf '%s: %d ms over %d cycles\n' "$mode" "$total" "$CYCLES"
}

for mode in full always auto; do
  cp -a "$SRC" "$WORK/$mode"
  echo "=== $mode ==="
  ( run_session "$WORK/$mode" "$mode" )
done
