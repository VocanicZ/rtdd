#!/usr/bin/env bash
# The authoritative CI gate for this repo. .github/workflows/ci.yml runs the same
# checks; run this before merging so a green verdict never depends on hosted CI.
#
# Go 1.24+ and uv must be on PATH — see DEVELOPMENT.md if either is not found.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> go build ./..."
go build ./...

echo "==> go vet ./..."
go vet ./...

echo "==> gofmt -l ."
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "gofmt found unformatted files:"
  echo "$unformatted"
  exit 1
fi

# -timeout 30m: on a saturated host internal/installtest overruns go test's default 10 min
# timeout (#514). Its tests are correctness gates, so they get time, never a skip.
echo "==> go test ./... -count=1"
go test ./... -count=1 -timeout 30m

echo "==> node --test installer/"
node --test installer/

echo "==> rtdd-gen check"
go run ./cmd/rtdd-gen check

echo "==> rtdd-gen verify"
go run ./cmd/rtdd-gen verify

echo "==> embedded protocol copy must match the source"
diff -u protocol/PROTOCOL.md internal/install/protocol.md

# Issue #496: the bench suites below shell out to whatever `rtdd` is first on PATH, so
# without this they test an ambient binary — a stale v0.2 install fails the gate on a
# green tree, and a stale build that happens to pass proves nothing about this one. The
# mirror of ci.yml's `rtdd on PATH` step (#377): the tool under test is the tree under test.
echo "==> rtdd on PATH"
rtdd_bin="$(mktemp -d)"
trap 'rm -rf "$rtdd_bin"' EXIT
go build -o "$rtdd_bin/rtdd" ./cmd/rtdd
export PATH="$rtdd_bin:$PATH"

echo "==> prereg gate"
scripts/ci-prereg.sh

# Issue #340: bench/ (Axis 2 replay) is a separate uv project from bench/swebench, and
# until this milestone neither gate ran it. It now holds the derivation that produces
# bench/results/*/summary.{json,md}, so a change to derive.py, report.py or metrics.py
# could otherwise go green through both gates while breaking every published table.
echo "==> bench replay gate"
(cd bench && uv sync && uv run pytest -q && uv run python -m replay.cli audit)

echo "==> static binary"
CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd
file /tmp/rtdd
file /tmp/rtdd | grep -q 'statically linked'

# The host build above covers linux/amd64 only. PRD #6 criterion 5 wants all four
# released artifacts, so cross-build the whole .goreleaser.yaml matrix and inspect
# each one by executable format (ELF, Mach-O, PE).
echo "==> every release artifact is statically linked"
go test -count=1 -run '^TestReleaseArtifactsAreStaticallyLinked$' ./internal/installtest/ -timeout 30m

# budget_step runs one wall-clock budget test as its own named step. The `-list` guard
# keeps a renamed or deleted test from passing as zero executed tests. A budget test skips
# itself with an INCONCLUSIVE: message when the host is saturated (internal/testenv, #514):
# CPU pressure (PSI `some avg10`) at or above testenv.PSIThreshold, or load1 above the CPU
# count (#540). A timing taken on an oversubscribed box is not a measurement, so it neither passes nor
# fails the budget, and the step says so and carries on. A real --- FAIL still fails the
# gate: the output is captured, not piped, so go test's own exit status decides the step.
budget_step() {
  local name="$1" pkg="$2" listed out reason
  listed="$(go test -list "^$name\$" "$pkg")"
  case "$listed" in
    *"$name"*) ;;
    *) echo "the budget test $name is gone from $pkg"; exit 1 ;;
  esac
  if ! out="$(go test -count=1 -v -run "^$name\$" "$pkg" 2>&1)"; then
    echo "$out"
    exit 1
  fi
  echo "$out"
  case "$out" in
    *"--- SKIP: $name"*INCONCLUSIVE:*|*INCONCLUSIVE:*"--- SKIP: $name"*)
      reason="$(grep -m 1 -o 'INCONCLUSIVE: .*' <<<"$out")"
      echo "==> INCONCLUSIVE: $name (${reason#INCONCLUSIVE: })"
      ;;
  esac
}

# PRD #410 AC8 (#454): `rtdd which` on this repository with a warm graph cache is under
# 1 s, and Rounds of every line of it is too. `go test ./...` above runs these already;
# each step names its budget so a red build points straight at it.
echo "==> rtdd which stays interactive on this repository (#454)"
budget_step TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s ./cmd/rtdd/
echo "==> Rounds of every line of this repository stays under 1 s (#454)"
budget_step TestRoundsOfEveryLineOfThisRepositoryIsUnder1s ./internal/rounds/

# PRD #409 AC10: a cold scanner build of 10 000 files under 10 s, and a warm build of this
# repository under 1 s.
echo "==> cold scanner build of 10 000 files stays under 10 s (PRD #409 AC10)"
budget_step TestColdBuildOf10000FilesIsUnder10s ./internal/graphbuild/
echo "==> warm build of this repository stays under 1 s (PRD #409 AC10)"
budget_step TestWarmBuildOfThisRepositoryIsUnder1s ./internal/graphbuild/

# PRD #368 AC11 (#380): the archives a release would publish, and install.sh driven
# against them. Both tests below read build/dist, which .gitignore ignores — on a clean
# checkout nothing fills it, so the snapshot build is what makes them mean anything. It
# publishes NOTHING: --snapshot creates no tag, no release and no draft, it re-runs no
# benchmark and it primes no bench cache.
#
# --skip=before because .goreleaser.yaml's before hooks are `go mod tidy`, the two
# rtdd-gen commands and `go test ./...` — every one of them already its own step above.
# Running them a second time inside GoReleaser proves nothing new and doubles the suite.
echo "==> release snapshot archives (#368 AC7)"
scripts/release-snapshot.sh --skip=before

# The `-list` line in front of each `-run` is this repo's idiom, and it is load-bearing
# here: `go test -run` on a pattern that matches nothing exits 0, so a deleted or renamed
# gate would sail through as a pass over zero executed tests.
echo "==> release archives ship every shipped path (#368 AC7)"
listed="$(go test -list '^TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath$' ./internal/installtest/)"
case "$listed" in
  *TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath*) ;;
  *) echo "the release-archive contents gate is gone from internal/installtest"; exit 1 ;;
esac
go test -count=1 -run '^TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath$' ./internal/installtest/ -timeout 30m

echo "==> installer end to end against the real snapshot archives (#368 AC8)"
listed="$(go test -list '^TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion$' ./internal/installtest/)"
case "$listed" in
  *TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion*) ;;
  *) echo "the end-to-end install gate is gone from internal/installtest"; exit 1 ;;
esac
go test -count=1 -run '^TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion$' ./internal/installtest/ -timeout 30m

echo "==> ci-local: PASS"
