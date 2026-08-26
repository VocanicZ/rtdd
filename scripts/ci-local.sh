#!/usr/bin/env bash
# The authoritative CI gate for this repo. .github/workflows/ci.yml runs the same
# checks; run this before merging so a green verdict never depends on hosted CI.
#
# Go 1.24+ must be on PATH — see DEVELOPMENT.md if `go` is not found.
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

echo "==> static binary"
CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd
file /tmp/rtdd
file /tmp/rtdd | grep -q 'statically linked'

echo "==> ci-local: PASS"
