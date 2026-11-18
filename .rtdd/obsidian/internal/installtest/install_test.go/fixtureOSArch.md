---
id: "internal/installtest/install_test.go::fixtureOSArch"
source: "internal/installtest/install_test.go"
start: 150
end: 164
kind: "func"
is-test: false
tags: [rtdd/code]
---
# fixtureOSArch

#rtdd/code

## Tests

- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesResolvesTheLatestRelease]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum]] (calls)
- [[internal/installtest/install_test.go/TestInstallOnACleanPath]] (calls)
- [[internal/installtest/install_test.go/TestInstallAbortsOnACorruptedChecksum]] (calls)

## Callers

- [[internal/installtest/install_skill_test.go/installFixture]] (calls)
- [[internal/installtest/install_test.go/runUnpinnedInstall]] (calls)

## Callees

none
