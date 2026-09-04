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

echo "==> rtdd-gen check"
go run ./cmd/rtdd-gen check

echo "==> rtdd-gen verify"
go run ./cmd/rtdd-gen verify

echo "==> embedded protocol copy must match the source"
diff -u protocol/PROTOCOL.md internal/install/protocol.md

echo "==> prereg gate"
scripts/ci-prereg.sh

echo "==> static binary"
CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd
file /tmp/rtdd
file /tmp/rtdd | grep -q 'statically linked'

# The host build above covers linux/amd64 only. PRD #6 criterion 5 wants all four
# released artifacts, so cross-build the whole .goreleaser.yaml matrix and inspect
# each one by executable format (ELF, Mach-O, PE).
echo "==> every release artifact is statically linked"
go test -count=1 -run '^TestReleaseArtifactsAreStaticallyLinked$' .

echo "==> ci-local: PASS"
