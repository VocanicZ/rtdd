---
id: "internal/graph/graph.go::FileNode"
source: "internal/graph/graph.go"
start: 65
end: 71
kind: "func"
is-test: false
tags: [rtdd/code]
---
# FileNode

#rtdd/code

## Tests

- [[internal/rounds/fanout_test.go/TestAChangedWholeFileTestAddsNothingToRound2]] (calls)
- [[internal/rounds/filenodes_test.go/TestAChangedFileWithNoDefinitionSelectsTheTestsThatNameIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestAChangedFileNothingNamesIsUntested]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheTestsOfWhatLoadsAChangedFileAreRoundOne]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestFilesTopLevelReferenceSelectsItsTests]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestThatNamesAFileIsATestOfEveryChangeInIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheFileNodeOwnsOnlyLinesInNoDefinition]] (calls)
- [[internal/rounds/filenodes_test.go/TestLinksOfAFileListsATestFilesReferenceAsItsTests]] (calls)
- [[internal/rounds/prose_test.go/TestTopLevelChangeIgnoresTheCallersOfTheFilesFunctions]] (calls)
- [[internal/rounds/prose_test.go/TestAProseNodeDoesNotHopItsTestsIntoTheFileItNames]] (calls)
- [[internal/rounds/rounds_test.go/TestTopLevelLineIsTheModuleNodeAndTheTestsThatNameTheFile]] (calls)

## Callers

- [[internal/graphbuild/build.go/addFiles]] (calls)
- [[internal/rounds/fanout_test.go/fanoutGraph]] (calls)
- [[internal/scan/link.go/link_2]] (calls)

## Callees

none
