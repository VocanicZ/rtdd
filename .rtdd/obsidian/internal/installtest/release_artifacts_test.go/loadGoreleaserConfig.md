---
id: "internal/installtest/release_artifacts_test.go::loadGoreleaserConfig"
source: "internal/installtest/release_artifacts_test.go"
start: 70
end: 84
kind: "func"
is-test: false
tags: [rtdd/code]
---
# loadGoreleaserConfig

#rtdd/code

## Tests

- [[internal/installtest/release_artifacts_test.go/TestReleaseArtifactsAreStaticallyLinked]] (calls)
- [[internal/installtest/release_artifacts_test.go/TestReleaseBuildsAreConfiguredWithoutCgo]] (calls)
- [[internal/installtest/release_artifacts_test.go/TestReleaseArchivesShipTheLicenseAndReadme]] (calls)
- [[internal/installtest/release_artifacts_test.go/TestReleaseShipsBothWindowsArchiveFormats]] (calls)
- [[internal/installtest/release_snapshot_test.go/TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath]] (calls)

## Callers

none

## Callees

- [[.goreleaser.yaml/_file]] (references)
- [[internal/installtest/release_preflight_test.go/findRepoRootForTest]] (calls)
