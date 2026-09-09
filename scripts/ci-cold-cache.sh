#!/usr/bin/env bash
# The bench gates, run the way a hosted runner runs them: no RTDD_BENCH_CACHE and no
# populated ~/.cache/rtdd-bench.
#
# .github/workflows/ci.yml's `prereg` job and its two bench steps have never been proven
# in that state — the workflow has been disabled since 2026-08-26, and every local run
# has had the ~201 MB store of collected suites, full-suite outcomes and repo mirrors
# sitting behind it. This script reproduces the bare-runner state and runs exactly those
# commands under it.
#
# The state is CHECKED before each pass, by `python -m replay.coldcache`, because getting
# it wrong is silent. `RTDD_BENCH_CACHE` moves the replay store and nothing else;
# `bench/swebench/run_arm.py` finds its store through `Path.home()`. Pointing the variable
# at an empty directory while leaving HOME alone yields a run that passes, reads the
# developer's warm store throughout, and proves nothing. So HOME is moved too, and both
# stores are counted before a single gate runs.
#
# The real ~/.cache/rtdd-bench is never deleted, re-keyed or invalidated — that is #218,
# an open human decision. This script only counts files under it, before and after, and
# fails if the count or the byte total moved.
#
# Go 1.24+ and uv must be on PATH — see DEVELOPMENT.md if either is not found.
set -euo pipefail

cd "$(dirname "$0")/.."
repo="$PWD"

if ! command -v uv >/dev/null 2>&1; then
  echo "uv not found on PATH — install it, see DEVELOPMENT.md" >&2
  exit 1
fi

real_home="${HOME}"
warm="${real_home}/.cache/rtdd-bench"

# The warm store's fingerprint, so a run that touched it cannot end green.
fingerprint() {
  if [ -d "$1" ]; then
    find "$1" -type f -printf '%s\n' 2>/dev/null | awk '{n++; b+=$1} END {printf "%d files %d bytes\n", n+0, b+0}'
  else
    echo "absent"
  fi
}
warm_before="$(fingerprint "$warm")"
echo "==> warm store before: $warm — $warm_before"

cold="$(mktemp -d "${TMPDIR:-/tmp}/rtdd-cold-cache.XXXXXX")"
trap 'rm -rf "$cold"' EXIT
mkdir -p "$cold/home" "$cold/replay-cache"

# uv's own package cache and managed interpreters are NOT the thing under test — the
# hosted runner caches them too (`astral-sh/setup-uv` with `enable-cache: true`). Keep
# them pointed at the real home so a cold HOME does not turn this into a download test.
export UV_CACHE_DIR="${UV_CACHE_DIR:-${real_home}/.cache/uv}"
export UV_PYTHON_INSTALL_DIR="${UV_PYTHON_INSTALL_DIR:-${real_home}/.local/share/uv/python}"

# Every `uv run pytest` below adds `-rs` to the workflow's `-q` so any test that skips
# prints its reason into this log: #371 wants a skip that names the cache, not a skip
# that is merely counted.
cold_guard() {
  echo "==> coldness check ($1)"
  (cd "$repo/bench" && uv run python -m replay.coldcache)
}

# --- pass 1: RTDD_BENCH_CACHE at an empty temp directory --------------------------
echo
echo "=== pass 1 — RTDD_BENCH_CACHE=$cold/replay-cache, HOME=$cold/home ==="
(
  export HOME="$cold/home"
  export RTDD_BENCH_CACHE="$cold/replay-cache"
  cold_guard "pass 1"

  cd "$repo/bench"
  echo "==> uv sync"
  uv sync
  echo "==> uv run pytest -q  (bench replay suite)"
  uv run pytest -q -rs
  echo "==> uv run python -m replay.cli audit  (corpus admission gate)"
  uv run python -m replay.cli audit
)

# --- pass 2: RTDD_BENCH_CACHE unset, the default bench/cache path cold -------------
echo
echo "=== pass 2 — RTDD_BENCH_CACHE unset, HOME=$cold/home ==="
if [ -d "$repo/bench/cache" ] && [ -n "$(find "$repo/bench/cache" -type f -print -quit)" ]; then
  echo "bench/cache/ holds entries; pass 2 needs the default path cold. Move it aside." >&2
  exit 1
fi
(
  export HOME="$cold/home"
  unset RTDD_BENCH_CACHE
  cold_guard "pass 2"

  cd "$repo/bench"
  echo "==> uv run pytest -q  (bench replay suite)"
  uv run pytest -q -rs
  echo "==> uv run python -m replay.cli audit  (corpus admission gate)"
  uv run python -m replay.cli audit

  cd "$repo/bench/swebench"
  echo "==> uv sync"
  uv sync
  echo "==> uv run pytest -q  (swebench suite)"
  uv run pytest -q -rs
  echo "==> uv run python run_arm.py --dry-run  (arm-driver dry run)"
  uv run python run_arm.py --dry-run >/dev/null
  echo "==> scripts/ci-prereg.sh  (prereg gate)"
  "$repo/scripts/ci-prereg.sh"
)

# --- what the run must not have done ----------------------------------------------
echo
warm_after="$(fingerprint "$warm")"
echo "==> warm store after: $warm — $warm_after"
if [ "$warm_before" != "$warm_after" ]; then
  echo "the cold-cache run changed $warm ($warm_before -> $warm_after); #218 keeps it untouched" >&2
  exit 1
fi

dirty="$(git -C "$repo" status --porcelain -- bench/results)"
if [ -n "$dirty" ]; then
  echo "the cold-cache run modified bench/results/ — no benchmark may be re-run:" >&2
  echo "$dirty" >&2
  exit 1
fi

echo "==> ci-cold-cache: PASS"
