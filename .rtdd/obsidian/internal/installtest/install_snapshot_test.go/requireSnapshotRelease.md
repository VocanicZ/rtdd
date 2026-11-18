---
id: "internal/installtest/install_snapshot_test.go::requireSnapshotRelease"
source: "internal/installtest/install_snapshot_test.go"
start: 87
end: 113
kind: "func"
is-test: false
tags: [rtdd/code]
---
# requireSnapshotRelease

#rtdd/code

## Tests

- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesResolvesTheLatestRelease]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum]] (calls)

## Callers

none

## Callees

- [[.gitignore/_file]] (references)
- [[.harness/.gitignore/_file]] (references)
- [[bench/.gitignore/_file]] (references)
- [[internal/installtest/install_snapshot_test.go/buildSnapshotRelease]] (calls)
- [[internal/installtest/release_preflight_test.go/findRepoRootForTest]] (calls)
- [[internal/installtest/release_snapshot_test.go/buildDistIsGitIgnored]] (calls)
- [[internal/installtest/release_snapshot_test.go/trackedTreeDigest]] (calls)
- [[internal/installtest/release_snapshot_test.go/moduleFetchFailure]] (calls)
