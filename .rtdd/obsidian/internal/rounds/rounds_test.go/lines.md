---
id: "internal/rounds/rounds_test.go::lines"
source: "internal/rounds/rounds_test.go"
start: 67
end: 69
kind: "func"
is-test: false
tags: [rtdd/code]
---
# lines

#rtdd/code

## Tests

- [[internal/rounds/fanout_test.go/TestAChangedProseFileAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedProseFileATestReadsIsRound1Only]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedDataFileAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedTestAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedWholeFileTestAddsNothingToRound2]] (calls)
- [[internal/rounds/filenodes_test.go/TestAChangedFileWithNoDefinitionSelectsTheTestsThatNameIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestAChangedFileNothingNamesIsUntested]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheTestsOfWhatLoadsAChangedFileAreRoundOne]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestFilesTopLevelReferenceSelectsItsTests]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestThatNamesAFileIsATestOfEveryChangeInIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheFileNodeOwnsOnlyLinesInNoDefinition]] (calls)
- [[internal/rounds/helpers_test.go/TestHelperIsTransparentToTheTestsThatCallIt]] (calls)
- [[internal/rounds/helpers_test.go/TestHelperNoTestCallsSelectsItsWholeFile]] (calls)
- [[internal/rounds/helpers_test.go/TestWholeFileTestIsSelectedOnceByItsPath]] (calls)
- [[internal/rounds/helpers_test.go/TestRoundListsEachRunnableIDOnce]] (calls)
- [[internal/rounds/prose_test.go/TestTopLevelChangeIgnoresTheCallersOfTheFilesFunctions]] (calls)
- [[internal/rounds/prose_test.go/TestAProseNodeDoesNotHopItsTestsIntoTheFileItNames]] (calls)
- [[internal/rounds/rounds_test.go/TestInnermostNodeOwnsAChangedLine]] (calls)
- [[internal/rounds/rounds_test.go/TestTopLevelLineIsTheModuleNodeAndTheTestsThatNameTheFile]] (calls)
- [[internal/rounds/rounds_test.go/TestALineInAFileWithNoNodeIsNoChangedNode]] (calls)
- [[internal/rounds/rounds_test.go/TestAChangedTestNodeIsInRound1]] (calls)
- [[internal/rounds/rounds_test.go/TestATopLevelLineOfATestFileSelectsItsTests]] (calls)
- [[internal/rounds/rounds_test.go/TestRound1IsTestsOfTheChangedNodeAndRound2TestsOfCallersAndCallees]] (calls)
- [[internal/rounds/rounds_test.go/TestEveryRelationIsTraversedInBothDirections]] (calls)
- [[internal/rounds/rounds_test.go/TestATestNodeIsNeverANeighbour]] (calls)
- [[internal/rounds/rounds_test.go/TestRound2ExcludesRound1]] (calls)
- [[internal/rounds/rounds_test.go/TestUntestedIsTheChangedNodesNoRoundReaches]] (calls)
- [[internal/rounds/rounds_test.go/TestRoundsAreOrderedByFileThenLineAndDeterministic]] (calls)
- [[internal/rounds/rounds_test.go/TestOwnerOfTwoNodesWithOneSpanIsTheSameInEveryOrder]] (calls)

## Callers

none

## Callees

none
