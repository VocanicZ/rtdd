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

echo "==> go test ./... -count=1"
go test ./... -count=1

echo "==> node --test installer/"
node --test installer/

echo "==> rtdd-gen check"
go run ./cmd/rtdd-gen check

echo "==> rtdd-gen verify"
go run ./cmd/rtdd-gen verify

echo "==> embedded protocol copy must match the source"
diff -u protocol/PROTOCOL.md internal/install/protocol.md

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
go test -count=1 -run '^TestReleaseArtifactsAreStaticallyLinked$' .

# PRD #232 AC11: the shipped adapter set's two gates, named as their own steps so a red
# build points straight at the adapter set. `go test ./...` above runs both already; what
# is added here is the `-list` line in front of each, because `go test -run` on a pattern
# that matches nothing exits 0 — a deleted or renamed gate would otherwise pass silently.
echo "==> shipped-adapter completeness gate (#310)"
listed="$(go test -list '^TestEveryShippedAdapterIsFullySpecified$' ./internal/contract/)"
case "$listed" in
  *TestEveryShippedAdapterIsFullySpecified*) ;;
  *) echo "the shipped-adapter completeness gate is gone from ./internal/contract/"; exit 1 ;;
esac
go test -count=1 -run '^TestEveryShippedAdapterIsFullySpecified$' ./internal/contract/

echo "==> shipped id_template round-trip over real captured output (#315)"
listed="$(go test -list 'RoundTrip' ./internal/report/)"
case "$listed" in
  *RoundTrip*) ;;
  *) echo "./internal/report/ has no id round-trip test left"; exit 1 ;;
esac
go test -count=1 -run 'RoundTrip' ./internal/report/

# Issue #334 AC2: the TS tier names test FILES and six shipped runners select by test NAME.
# The gate below is the only thing standing between that mismatch and a green run over zero
# executed tests, so it gets its own step — with the `-list` line in front, because
# `go test -run` on a pattern that matches nothing exits 0 and a deleted gate would pass
# silently.
echo "==> every shipped adapter's TS selection matches its own subset selector (#334)"
listed="$(go test -list '^TestEveryShippedAdapterSelectionMatchesItsSubsetSelector$' ./cmd/rtdd/)"
case "$listed" in
  *TestEveryShippedAdapterSelectionMatchesItsSubsetSelector*) ;;
  *) echo "the selection/selector gate is gone from ./cmd/rtdd/"; exit 1 ;;
esac
go test -count=1 -run '^TestEveryShippedAdapterSelectionMatchesItsSubsetSelector$|^TestEveryStaticShippedAdapterHasASelectorCase$|^TestEveryStaticShippedAdapterDeclaresATestSelector$|^TestGoFixtureSelectionActuallyExecutesTestAdd$' ./cmd/rtdd/

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
listed="$(go test -list '^TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath$' .)"
case "$listed" in
  *TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath*) ;;
  *) echo "the release-archive contents gate is gone from the root package"; exit 1 ;;
esac
go test -count=1 -run '^TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath$' .

echo "==> installer end to end against the real snapshot archives (#368 AC8)"
listed="$(go test -list '^TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion$' .)"
case "$listed" in
  *TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion*) ;;
  *) echo "the end-to-end install gate is gone from the root package"; exit 1 ;;
esac
go test -count=1 -run '^TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion$' .

echo "==> ci-local: PASS"
