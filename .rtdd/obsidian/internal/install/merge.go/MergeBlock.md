---
id: "internal/install/merge.go::MergeBlock"
source: "internal/install/merge.go"
start: 70
end: 103
kind: "func"
is-test: false
tags: [rtdd/code]
---
# MergeBlock

#rtdd/code

## Tests

- [[internal/install/merge_test.go/TestMergeIntoAnEmptyFileIsTheBlockAlone]] (calls)
- [[internal/install/merge_test.go/TestMergeAppendsToAFileWithNoMarkersAndKeepsEverything]] (calls)
- [[internal/install/merge_test.go/TestMergeReplacesOnlyBetweenTheMarkers]] (calls)
- [[internal/install/merge_test.go/TestMergeIsIdempotent]] (calls)
- [[internal/install/merge_test.go/TestUnterminatedMarkerIsAnErrorNotAGuess]] (calls)
- [[internal/install/merge_test.go/TestDoubledBeginMarkerIsAnError]] (calls)
- [[internal/install/uninstall_test.go/TestMergeThenRemoveReturnsTheFileUnchanged]] (calls)

## Callers

- [[internal/install/install.go/Plan]] (calls)
- [[internal/install/uninstall_test.go/installedRepo]] (calls)

## Callees

none
