---
id: ".goreleaser.yaml"
source: ".goreleaser.yaml"
start: 1
end: 73
kind: "file"
is-test: false
tags: [rtdd/file]
---
# .goreleaser.yaml

#rtdd/file

## Tests

- [[internal/contract/plan_m7_test.go/TestM7PlanStatesTheTwoFactsItsTasksTurnOn]] (references)
- [[internal/contract/releasing_doc_test.go/TestReleasingDocNamesTheDraftReleaseTrap]] (references)
- [[internal/installtest/release_artifacts_test.go/TestReleaseArtifactsAreStaticallyLinked]] (references)
- [[internal/installtest/release_artifacts_test.go/TestReleaseArchivesShipTheLicenseAndReadme]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestReleaseSnapshotScriptWritesOnlyToTheIgnoredDistDir]] (references)
- [[internal/installtest/release_snapshot_script_test.go/TestNoOtherScriptInvokesGoreleaser]] (references)
- [[internal/installtest/release_snapshot_test.go/TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[docs/RELEASING.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/07-m7-release-readiness.md/_file]] (references)
- [[docs/plans/10-rounds-cutover.md/_file]] (references)
- [[internal/installtest/install_snapshot_test.go/buildSnapshotRelease]] (references)
- [[internal/installtest/release_artifacts_test.go/loadGoreleaserConfig]] (references)
- [[internal/installtest/release_snapshot_test.go/archiveShippedFiles]] (references)

## Callees

- [[README.md/_file]] (references)
- [[bench/results/README.md/_file]] (references)
- [[bench/results/paired/flask/README.md/_file]] (references)
- [[docs/outcomes/README.md/_file]] (references)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
- [[internal/protocol/testdata/golden/GLOBAL-AGENTS.md/_file]] (references)
- [[internal/protocol/testdata/golden/GLOBAL-SKILL.md/_file]] (references)
- [[internal/protocol/testdata/golden/SKILL.md/_file]] (references)
- [[internal/protocol/testdata/golden/rtdd.mdc/_file]] (references)
