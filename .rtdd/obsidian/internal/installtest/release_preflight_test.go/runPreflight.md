---
id: "internal/installtest/release_preflight_test.go::runPreflight"
source: "internal/installtest/release_preflight_test.go"
start: 149
end: 173
kind: "func"
is-test: false
tags: [rtdd/code]
---
# runPreflight

#rtdd/code

## Tests

- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsUnreadyRepoAndRefusesToExitZero]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsFullyGreenRepo]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightFailsOnAnUnverifiedReleaseArtifact]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsFourPassesOnTheRepoAsItStandsToday]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightStillRefusesTheTwoIrreversibleActions]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightLeavesTheRepositoryUntouched]] (calls)

## Callers

none

## Callees

- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
- [[internal/installtest/release_artifacts_test.go/String]] (calls)
- [[internal/installtest/release_preflight_test.go/writeToolStubs]] (calls)
- [[scripts/release-preflight.sh/_file]] (references)
