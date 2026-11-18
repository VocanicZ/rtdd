---
id: "scripts/release-snapshot.sh"
source: "scripts/release-snapshot.sh"
start: 1
end: 41
kind: "file"
is-test: false
tags: [rtdd/file]
---
# release-snapshot.sh

#rtdd/file

## Tests

- [[internal/contract/ci_workflow_test.go/TestCIWorkflowIsRecordedActive]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIWorkflowStillTriggersOnPushToMainAndOnPullRequests]] (references)
- [[internal/contract/ci_workflow_test.go/TestCITestJobConcludedSuccessOnAMainCommit]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIBenchStepsRanWithoutAPrimedCache]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIWorkflowPrimesNoBenchCache]] (references)
- [[internal/contract/ci_workflow_test.go/TestBothGatesRunTheReleaseSnapshotAndItsTests]] (references)
- [[internal/contract/ci_workflow_test.go/TestREADMEBaselineComparisonPageExists]] (references)
- [[internal/contract/ci_workflow_test.go/TestLocalCIPutsTheCheckoutsRtddFirstOnPathBeforeTheBenchGates]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestReleaseSnapshotScriptIsExecutable]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestReleaseSnapshotScriptFetchesGoreleaserItselfAtAPinnedVersion]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestReleaseSnapshotScriptCreatesNoTagReleaseOrDraft]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestReleaseSnapshotScriptWritesOnlyToTheIgnoredDistDir]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestNoOtherScriptInvokesGoreleaser]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestTheArchiveGateNeverSkipsForAMissingBinary]] (references)

## Callers

- [[.github/workflows/ci.yml/_file]] (references)
- [[docs/plans/07-m7-release-readiness.md/_file]] (references)
- [[scripts/ci-local.sh/_file]] (references)

## Callees

none
