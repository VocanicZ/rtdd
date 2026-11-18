---
id: "internal/install/merge_test.go::block"
source: "internal/install/merge_test.go"
start: 10
end: 12
kind: "func"
is-test: false
tags: [rtdd/code]
---
# block

#rtdd/code

## Tests

- [[internal/install/legacy_test.go/TestLegacyFilesStripTheOldBlockAndLeaveTheUsersTextByteIdentical]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesLeaveOptInsAndForeignFilesAlone]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesStripTheBlockFromTheSharedHomeFiles]] (calls)
- [[internal/install/legacy_test.go/TestReportLegacyIsAReadOnlyLinePerPiece]] (calls)
- [[internal/install/merge_test.go/TestMergeIntoAnEmptyFileIsTheBlockAlone]] (calls)
- [[internal/install/merge_test.go/TestMergeAppendsToAFileWithNoMarkersAndKeepsEverything]] (calls)
- [[internal/install/merge_test.go/TestMergeReplacesOnlyBetweenTheMarkers]] (calls)
- [[internal/install/merge_test.go/TestMergeIsIdempotent]] (calls)
- [[internal/install/merge_test.go/TestUnterminatedMarkerIsAnErrorNotAGuess]] (calls)
- [[internal/install/merge_test.go/TestDoubledBeginMarkerIsAnError]] (calls)

## Callers

- [[internal/install/install_test.go/fakeFiles]] (calls)

## Callees

none
