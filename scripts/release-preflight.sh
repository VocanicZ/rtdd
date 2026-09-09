#!/usr/bin/env bash
# release-preflight.sh — plan Task 24's agent half (issue #201): the checklist
# command that runs every automated release check, prints the
# "DECISION REQUIRED — repository visibility and release" block from
# docs/plans/05-m4-m5-swebench-release.md Task 24 with every field filled in
# from what it just ran, and stops.
#
# It NEVER mutates repository state: no `gh repo edit`, no `git tag`, no
# `git push`, no `gh release`. Whether to go public and whether to push a
# release tag are irreversible calls that stay with a human (plan Task 24).
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1

fail=0
banner() { printf '\n==> %s\n' "$1"; }

# --- 1. front-end generator: dist/ freshness and validity -----------------
banner "go run ./cmd/rtdd-gen check"
if go run ./cmd/rtdd-gen check; then
  check_status=pass
else
  echo "FAIL: rtdd-gen check" >&2
  check_status=fail
  fail=1
fi

banner "go run ./cmd/rtdd-gen verify"
if go run ./cmd/rtdd-gen verify; then
  verify_status=pass
else
  echo "FAIL: rtdd-gen verify" >&2
  verify_status=fail
  fail=1
fi

frontend_status=pass
if [ "$check_status" != pass ] || [ "$verify_status" != pass ]; then
  frontend_status=fail
fi

# --- 2. go test ./... -------------------------------------------------------
banner "go test ./..."
if go test ./...; then
  gotest_status=pass
else
  echo "FAIL: go test ./..." >&2
  gotest_status=fail
  fail=1
fi

# --- 3. release artifacts: every published binary is self-contained --------
# PRD #6 acceptance criterion 5. The check lives in a Go test so it needs no extra
# tooling and gives the same verdict here as in CI; it is reported on its own line
# because a repo can have a green suite and still produce an artifact that will not
# run on a user's machine.
banner "go test -run TestReleaseArtifactsAreStaticallyLinked"
if go test -count=1 -run '^TestReleaseArtifactsAreStaticallyLinked$' .; then
  linkage_status=pass
else
  echo "FAIL: release artifacts are not all statically linked" >&2
  linkage_status=fail
  fail=1
fi

# --- 4. bench/swebench: pytest, then the pre-registration launch gate ------
# These are two different kinds of thing and are reported as two different lines.
# `pytest -q` is a test suite. `preflight.py` is the M4 *launch gate*: it decides
# whether an arm may spend a token, and it refuses with exit 3 for as long as
# bench/PREREGISTRATION.md is unsigned. scripts/ci-prereg.sh treats exactly that
# refusal as a PASS, because an unsigned pre-registration is the correct state until
# a human writes the kill criterion and signs. Folding it into "Test suite" reported
# a decision nobody has taken yet as a broken test suite.
pytest_status=pass
launch_gate_status=pass
if ! command -v uv >/dev/null 2>&1; then
  banner "bench/swebench pytest + preflight"
  echo "uv not found on PATH — see DEVELOPMENT.md" >&2
  pytest_status=missing
  launch_gate_status="unknown (uv missing — preflight.py not run)"
  fail=1
else
  banner "cd bench/swebench && uv run pytest -q"
  if ! (cd bench/swebench && uv run pytest -q); then
    echo "FAIL: bench/swebench pytest -q" >&2
    pytest_status=fail
    launch_gate_status="skipped (bench/swebench pytest failed)"
    fail=1
  else
    banner "cd bench/swebench && uv run python preflight.py"
    preflight_out="$(cd bench/swebench && uv run python preflight.py 2>&1)"
    preflight_code=$?
    printf '%s\n' "$preflight_out"
    if [ "$preflight_code" -eq 0 ]; then
      launch_gate_status="pass (pre-registration signed; the M4 run may launch)"
    elif [ "$preflight_code" -eq 3 ] && ! grep -q '^status: SIGNED' bench/PREREGISTRATION.md; then
      # The gate did its job. Nothing here is broken and nothing here is for an agent
      # to fix: signing is the human's call, like the two actions below.
      launch_gate_status="refused (pre-registration unsigned — correct; a human signs it)"
      fail=1
    else
      echo "FAIL: bench/swebench preflight.py (exit $preflight_code)" >&2
      launch_gate_status=fail
      fail=1
    fi
  fi
fi

if [ "$pytest_status" = missing ]; then
  test_suite_status="unknown (uv missing — bench/swebench pytest not run)"
elif [ "$gotest_status" = fail ] || [ "$pytest_status" = fail ]; then
  test_suite_status=fail
else
  test_suite_status=pass
fi

# --- 5. placeholder grep ----------------------------------------------------
banner "placeholder grep"
placeholder_hits="$(grep -rn -E 'TBD|TODO|FIXME|REPLACE_WITH' README.md protocol/PROTOCOL.md dist/ bench/PREREGISTRATION.md 2>/dev/null || true)"
if [ -n "$placeholder_hits" ]; then
  printf '%s\n' "$placeholder_hits"
  fail=1
else
  echo "no placeholders"
fi

# --- 6. prereg-m4 tag: sha and date -----------------------------------------
banner "prereg-m4 tag"
if tag_line="$(git log -1 --format='%h %ad' --date=short prereg-m4 2>/dev/null)"; then
  prereg_tag_status="prereg-m4 -> $tag_line"
  echo "$prereg_tag_status"
else
  prereg_tag_status="prereg-m4 -> missing"
  echo "$prereg_tag_status (no such tag)"
fi

# --- 7. current repository visibility ---------------------------------------
banner "repo visibility"
if visibility_raw="$(gh repo view VocanicZ/rtdd --json visibility -q .visibility 2>/dev/null)"; then
  visibility_status="$(printf '%s' "$visibility_raw" | tr '[:upper:]' '[:lower:]')"
  echo "$visibility_status"
else
  echo "gh repo view failed — could not determine visibility" >&2
  visibility_status=unknown
fi

# --- 8. kill criterion, from the M4 tables if they exist --------------------
tables_md="bench/results/swebench/tables.md"
if [ -f "$tables_md" ]; then
  kc_line="$(grep -oE 'Result: \*\*(MET|NOT MET)\*\*' "$tables_md" | head -1)"
  if [ -n "$kc_line" ]; then
    kill_status="$(printf '%s' "$kc_line" | sed -E 's/Result: \*\*(.*)\*\*/\1/')"
  else
    kill_status="unknown ($tables_md present but no kill-criterion line found)"
  fi
else
  kill_status="unknown (no $tables_md — the M4 run has not completed)"
fi

# --- 9. which branch Task 14 selected ---------------------------------------
if [ -f docs/outcomes/SELECTED ]; then
  branch_taken="$(tr -d '[:space:]' < docs/outcomes/SELECTED)"
else
  branch_taken=unknown
fi

# --- print the decision block and stop --------------------------------------
banner "DECISION REQUIRED"
cat <<EOF

DECISION REQUIRED — repository visibility and release

Branch taken at Task 14: $branch_taken
Kill criterion:        $kill_status
Front-end checks:      $frontend_status
Test suite:            $test_suite_status
Release binaries:      $linkage_status
Placeholders:          ${placeholder_hits:-none}
Pre-registration tag:  $prereg_tag_status
M4 launch gate:        $launch_gate_status
Current visibility:    $visibility_status

Two irreversible actions need your explicit go-ahead, separately:

  1. Make the repository public. This publishes the benchmark results, the raw
     per-instance records, the pre-registration, and the prior-art claims about TDAD,
     pytest-testmon, Wallaby, Infinitest, Ekstazi, and SonarQube. Read the README's
     prior-art section once more before saying yes — it names other people's work.

  2. Push a v* tag, which triggers GoReleaser and publishes binaries plus the
     curl | sh install path.
     <If the negative branch was taken, action 2 is off the table: releases are
     disabled and spec §15 says do not ship a competitor.>

I will not do either of these. Tell me which, if any, to proceed with.
EOF

exit "$fail"
