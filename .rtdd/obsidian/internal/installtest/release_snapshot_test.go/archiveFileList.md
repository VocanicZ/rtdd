---
id: "internal/installtest/release_snapshot_test.go::archiveFileList"
source: "internal/installtest/release_snapshot_test.go"
start: 110
end: 119
kind: "func"
is-test: false
tags: [rtdd/code]
---
# archiveFileList

#rtdd/code

## Tests

- [[internal/installtest/release_snapshot_test.go/TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath]] (calls)
- [[internal/installtest/release_snapshot_test.go/TestArchiveContentCheckRejectsAnArchiveMissingAShippedPath]] (calls)
- [[internal/installtest/release_snapshot_test.go/TestArchiveReaderRejectsTheWrongFormat]] (calls)

## Callers

none

## Callees

- [[internal/installtest/release_snapshot_test.go/zipFileList]] (calls)
- [[internal/installtest/release_snapshot_test.go/tarGzFileList]] (calls)
