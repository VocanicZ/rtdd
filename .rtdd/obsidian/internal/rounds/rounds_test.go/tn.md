---
id: "internal/rounds/rounds_test.go::tn"
source: "internal/rounds/rounds_test.go"
start: 27
end: 31
kind: "func"
is-test: false
tags: [rtdd/code]
---
# tn

#rtdd/code

## Tests

- [[internal/rounds/fanout_test.go/TestAChangedWholeFileTestAddsNothingToRound2]] (calls)
- [[internal/rounds/filenodes_test.go/TestAChangedFileWithNoDefinitionSelectsTheTestsThatNameIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheTestsOfWhatLoadsAChangedFileAreRoundOne]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestFilesTopLevelReferenceSelectsItsTests]] (calls)
- [[internal/rounds/helpers_test.go/TestHelperIsTransparentToTheTestsThatCallIt]] (calls)
- [[internal/rounds/helpers_test.go/TestHelperNoTestCallsSelectsItsWholeFile]] (calls)
- [[internal/rounds/helpers_test.go/TestRoundListsEachRunnableIDOnce]] (calls)
- [[internal/rounds/links_test.go/TestLinksOfSplitsTestsCallersAndCallees]] (calls)
- [[internal/rounds/links_test.go/TestLinkerMatchesLinksOf]] (calls)
- [[internal/rounds/prose_test.go/TestTopLevelChangeIgnoresTheCallersOfTheFilesFunctions]] (calls)
- [[internal/rounds/prose_test.go/TestAProseNodeDoesNotHopItsTestsIntoTheFileItNames]] (calls)
- [[internal/rounds/rounds_test.go/TestTopLevelLineIsTheModuleNodeAndTheTestsThatNameTheFile]] (calls)
- [[internal/rounds/rounds_test.go/TestAChangedTestNodeIsInRound1]] (calls)
- [[internal/rounds/rounds_test.go/TestATopLevelLineOfATestFileSelectsItsTests]] (calls)
- [[internal/rounds/rounds_test.go/TestRound1IsTestsOfTheChangedNodeAndRound2TestsOfCallersAndCallees]] (calls)
- [[internal/rounds/rounds_test.go/TestEveryRelationIsTraversedInBothDirections]] (calls)
- [[internal/rounds/rounds_test.go/TestATestNodeIsNeverANeighbour]] (calls)
- [[internal/rounds/rounds_test.go/TestRound2ExcludesRound1]] (calls)
- [[internal/rounds/rounds_test.go/TestUntestedIsTheChangedNodesNoRoundReaches]] (calls)
- [[internal/rounds/rounds_test.go/TestRoundsAreOrderedByFileThenLineAndDeterministic]] (calls)

## Callers

- [[internal/rounds/fanout_test.go/fanoutGraph]] (calls)

## Callees

- [[internal/rounds/rounds_test.go/n]] (calls)
