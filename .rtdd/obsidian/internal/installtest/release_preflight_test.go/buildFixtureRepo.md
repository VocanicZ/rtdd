---
id: "internal/installtest/release_preflight_test.go::buildFixtureRepo"
source: "internal/installtest/release_preflight_test.go"
start: 97
end: 144
kind: "func"
is-test: false
tags: [rtdd/code]
---
# buildFixtureRepo

#rtdd/code

## Tests

- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsUnreadyRepoAndRefusesToExitZero]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsFullyGreenRepo]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightFailsOnAnUnverifiedReleaseArtifact]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsPlaceholdersAndMissingUvExplicitly]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsFourPassesOnTheRepoAsItStandsToday]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightStillRefusesTheTwoIrreversibleActions]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightLeavesTheRepositoryUntouched]] (calls)

## Callers

none

## Callees

- [[README.md/_file]] (references)
- [[bench/PREREGISTRATION.md/_file]] (references)
- [[bench/results/README.md/_file]] (references)
- [[bench/results/paired/flask/README.md/_file]] (references)
- [[docs/outcomes/README.md/_file]] (references)
- [[docs/outcomes/SELECTED/_file]] (references)
- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
- [[internal/gitctx/gittest/gittest.go/InitRepo]] (calls)
- [[internal/gitctx/gittest/gittest.go/HeadShort]] (calls)
- [[internal/installtest/release_preflight_test.go/findRepoRootForTest]] (calls)
- [[internal/protocol/testdata/golden/SKILL.md/_file]] (references)
- [[protocol/PROTOCOL.md/_file]] (references)
- [[scripts/release-preflight.sh/_file]] (references)
