---
id: "internal/graph/classify.go::WholeFileNode"
source: "internal/graph/classify.go"
start: 114
end: 116
kind: "func"
is-test: false
tags: [rtdd/code]
---
# WholeFileNode

#rtdd/code

## Tests

- [[internal/graph/split_test.go/TestWholeFileTestsFileWithNoNodeGetsItsFileNode]] (calls)
- [[internal/graph/split_test.go/TestRunnableID]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedWholeFileTestAddsNothingToRound2]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestFilesTopLevelReferenceSelectsItsTests]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestThatNamesAFileIsATestOfEveryChangeInIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestLinksOfAFileListsATestFilesReferenceAsItsTests]] (calls)
- [[internal/rounds/helpers_test.go/TestWholeFileTestIsSelectedOnceByItsPath]] (calls)

## Callers

- [[internal/graph/classify.go/SplitTests]] (calls)
- [[internal/rounds/rounds.go/Rounds]] (calls)

## Callees

none
