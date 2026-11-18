---
id: "cmd/rtdd/which_e2e_test.go::runE2EWhich"
source: "cmd/rtdd/which_e2e_test.go"
start: 94
end: 116
kind: "func"
is-test: false
tags: [rtdd/code]
---
# runE2EWhich

#rtdd/code

## Tests

- [[cmd/rtdd/init_shapes_test.go/TestWhichAfterAFreshInitNamesNothingUnderCursor]] (calls)
- [[cmd/rtdd/prose_test.go/TestWhichMapsAChangedProseFileToItsFileNodeAlone]] (calls)
- [[cmd/rtdd/which_comment_test.go/TestWhichIgnoresChangedLinesThatAreNotCode]] (calls)
- [[cmd/rtdd/which_e2e_test.go/TestWhichEndToEndEditingOneFunctionSelectsItsCallersAndNeighboursTests]] (calls)
- [[cmd/rtdd/which_e2e_test.go/TestWhichEndToEndEditingAFunctionWithNoLinkedTestListsItUntested]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichAChangelogAppendSelectsNothing]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichAChangedDocSelectsOnlyTheTestThatReadsIt]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichAChangedTestSelectsOnlyItself]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichNeverListsALogAsATest]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/TestWhichSelectsTheTestThatReadsAChangedFileWithNoDefinition]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/TestWhichListsAChangedFileNothingNamesAsUntested]] (calls)
- [[cmd/rtdd/which_wholefile_test.go/TestWhichWholeFileTestsListOnlyTheirPaths]] (calls)

## Callers

none

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[cmd/rtdd/which_rounds_test.go/tripwirePATH]] (calls)
