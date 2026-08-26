#!/usr/bin/env bash
# The pre-registration half of the CI gate: bench/swebench's prereg parser and
# the preflight refusal. .github/workflows/ci.yml's `prereg` job runs exactly
# these checks. bench/ itself (Axis 2 replay) is a separate uv project.
#
# uv must be on PATH — see DEVELOPMENT.md.
set -euo pipefail

cd "$(dirname "$0")/../bench/swebench"

if ! command -v uv >/dev/null 2>&1; then
  echo "uv not found on PATH — install it, see DEVELOPMENT.md" >&2
  exit 1
fi

echo "==> uv sync"
uv sync

echo "==> uv run pytest -q"
uv run pytest -q

echo "==> preflight gate"
# The gate must REFUSE while the pre-registration is unsigned, and PASS once a
# human has signed and tagged it. Either verdict is correct for its state; a
# refusal that is not exit 3, or a signed prereg that will not launch, is not.
set +e
uv run python preflight.py
code=$?
set -e
if grep -q '^status: SIGNED' ../PREREGISTRATION.md; then
  if [ "$code" -ne 0 ]; then
    echo "the pre-registration is SIGNED but preflight refused (exit $code)" >&2
    exit 1
  fi
else
  if [ "$code" -ne 3 ]; then
    echo "the pre-registration is unsigned; preflight must refuse with exit 3, got $code" >&2
    exit 1
  fi
  echo "unsigned pre-registration correctly refused"
fi

echo "==> ci-prereg: PASS"
