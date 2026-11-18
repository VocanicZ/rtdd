---
id: "internal/installtest/install_snapshot_test.go::TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum"
source: "internal/installtest/install_snapshot_test.go"
start: 349
end: 371
kind: "func"
is-test: true
tags: [rtdd/test]
---
# TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[internal/installtest/install_snapshot_test.go/releaseTag]] (calls)
- [[internal/installtest/install_snapshot_test.go/archiveFor]] (calls)
- [[internal/installtest/install_snapshot_test.go/requireSnapshotRelease]] (calls)
- [[internal/installtest/install_snapshot_test.go/serveSnapshotRelease]] (calls)
- [[internal/installtest/install_snapshot_test.go/goreleaserChecksums]] (calls)
- [[internal/installtest/install_snapshot_test.go/corruptChecksumFor]] (calls)
- [[internal/installtest/install_snapshot_test.go/assertOnlyTheFixtureServerWasAsked]] (calls)
- [[internal/installtest/install_test.go/runInstaller]] (calls)
- [[internal/installtest/install_test.go/fixtureOSArch]] (calls)
