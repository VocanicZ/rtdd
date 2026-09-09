#!/usr/bin/env bash
# release-snapshot.sh — plan 07 Task 5 Step 3: build the release archives locally,
# publishing nothing. This is the ONLY place this repo invokes GoReleaser.
#
# GoReleaser is fetched by `go run` at a pinned version rather than looked up on PATH, and
# `go run` does not touch go.mod. That is the whole point: release_snapshot_test.go's
# archive gate used to sit behind `exec.LookPath("goreleaser")`, and since nothing
# provisions GoReleaser on this host or on any CI runner, the gate skipped everywhere while
# the package still reported ok. A gate the repo can run for itself cannot be switched off
# by an unprovisioned host.
#
# It creates NOTHING on GitHub — no tag, no release, no draft (PRD #368's global
# constraint). --snapshot means GoReleaser needs no tag and refuses to publish, and
# --skip=publish says so a second time. Tagging, pushing, cutting a release and changing
# repository visibility are all absent from this file on purpose, and a test asserts they
# stay absent.
#
# Output goes to build/dist, which .goreleaser.yaml sets with `dist:` and .gitignore
# ignores. --dist is deliberately not overridden here: the default ./dist is the tracked
# generated-front-end tree, and --clean would wipe it.
#
# Idempotent: --clean wipes build/dist first, so a run's output is only that run's.
#
# Extra flags are forwarded. release_snapshot_test.go passes --skip=before, because
# .goreleaser.yaml's before hooks end with `go test ./...` and that test is one of them.
set -euo pipefail

cd "$(dirname "$0")/.."

# Pinned, never @latest: the gate asserts over these archives, so the toolchain that
# produces them is a version this repo chose. Resolve a newer one with
#   go list -m -versions github.com/goreleaser/goreleaser/v2
# and write the answer in.
GORELEASER="${GORELEASER:-github.com/goreleaser/goreleaser/v2@v2.12.7}"

exec go run "$GORELEASER" release --snapshot --clean --skip=publish "$@"
