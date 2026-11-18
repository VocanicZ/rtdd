---
id: "internal/installtest/release_preflight_test.go::findRepoRootForTest"
source: "internal/installtest/release_preflight_test.go"
start: 80
end: 87
kind: "func"
is-test: false
tags: [rtdd/code]
---
# findRepoRootForTest

#rtdd/code

## Tests

- [[internal/installtest/distcleanup_test.go/TestSuiteLeavesNoDistDirForGoreleaserToBuildInto]] (calls)
- [[internal/installtest/network_test.go/TestOnlySelfupdateReachesTheNetwork]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightNeverMutatesRepositoryState]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightIsDocumentedInDevelopmentMd]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightOnThisRepoReportsFourPasses]] (calls)
- [[internal/installtest/release_preflight_test.go/TestLinkageCheckEntrypointsNameTheTestsRealPackage]] (calls)
- [[internal/installtest/release_snapshot_script_test.go/TestReleaseSnapshotScriptIsExecutable]] (calls)
- [[internal/installtest/release_snapshot_script_test.go/TestNoOtherScriptInvokesGoreleaser]] (calls)
- [[internal/installtest/release_snapshot_script_test.go/TestTheArchiveGateNeverSkipsForAMissingBinary]] (calls)
- [[internal/installtest/release_snapshot_test.go/TestTheArchiveGateRunsWithNoGoreleaserBinaryOnPATH]] (calls)
- [[internal/installtest/release_snapshot_test.go/TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath]] (calls)

## Callers

- [[internal/installtest/install_snapshot_test.go/requireSnapshotRelease]] (calls)
- [[internal/installtest/install_snapshot_test.go/assertNothingLeakedOutsideTheTempInstallDir]] (calls)
- [[internal/installtest/release_artifacts_test.go/loadGoreleaserConfig]] (calls)
- [[internal/installtest/release_preflight_test.go/buildFixtureRepo]] (calls)
- [[internal/installtest/release_preflight_test.go/realPreregContent]] (calls)
- [[internal/installtest/release_snapshot_script_test.go/readSnapshotScript]] (calls)

## Callees

none
