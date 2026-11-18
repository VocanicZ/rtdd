---
id: "internal/rounds/rounds_test.go::n"
source: "internal/rounds/rounds_test.go"
start: 18
end: 24
kind: "func"
is-test: false
tags: [rtdd/code]
---
# n

#rtdd/code

## Tests

- [[cmd/rtdd/readme_example_test.go/TestREADMEShowsOnlyCommandsRtddHas]] (calls)
- [[internal/contract/plan_m7_test.go/TestM7PlanClaimsEveryPRDAcceptanceCriterionExactlyOnce]] (calls)
- [[internal/graph/split_test.go/TestRunnableID]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedWholeFileTestAddsNothingToRound2]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheTestsOfWhatLoadsAChangedFileAreRoundOne]] (calls)
- [[internal/rounds/filenodes_test.go/TestATestThatNamesAFileIsATestOfEveryChangeInIt]] (calls)
- [[internal/rounds/filenodes_test.go/TestTheFileNodeOwnsOnlyLinesInNoDefinition]] (calls)
- [[internal/rounds/helpers_test.go/TestHelperIsTransparentToTheTestsThatCallIt]] (calls)
- [[internal/rounds/helpers_test.go/TestHelperNoTestCallsSelectsItsWholeFile]] (calls)
- [[internal/rounds/helpers_test.go/TestWholeFileTestIsSelectedOnceByItsPath]] (calls)
- [[internal/rounds/helpers_test.go/TestRoundListsEachRunnableIDOnce]] (calls)
- [[internal/rounds/links_test.go/TestLinksOfSplitsTestsCallersAndCallees]] (calls)
- [[internal/rounds/links_test.go/TestLinksOfAnUnknownNodeIsEmptyNotNil]] (calls)
- [[internal/rounds/links_test.go/TestLinkerMatchesLinksOf]] (calls)
- [[internal/rounds/prose_test.go/TestTopLevelChangeIgnoresTheCallersOfTheFilesFunctions]] (calls)
- [[internal/rounds/prose_test.go/TestAProseNodeDoesNotHopItsTestsIntoTheFileItNames]] (calls)
- [[internal/rounds/rounds_test.go/TestInnermostNodeOwnsAChangedLine]] (calls)
- [[internal/rounds/rounds_test.go/TestOwnerIsTheInnermostNodeOfALine]] (calls)
- [[internal/rounds/rounds_test.go/TestTopLevelLineIsTheModuleNodeAndTheTestsThatNameTheFile]] (calls)
- [[internal/rounds/rounds_test.go/TestALineInAFileWithNoNodeIsNoChangedNode]] (calls)
- [[internal/rounds/rounds_test.go/TestRound1IsTestsOfTheChangedNodeAndRound2TestsOfCallersAndCallees]] (calls)
- [[internal/rounds/rounds_test.go/TestEveryRelationIsTraversedInBothDirections]] (calls)
- [[internal/rounds/rounds_test.go/TestATestNodeIsNeverANeighbour]] (calls)
- [[internal/rounds/rounds_test.go/TestRound2ExcludesRound1]] (calls)
- [[internal/rounds/rounds_test.go/TestUntestedIsTheChangedNodesNoRoundReaches]] (calls)
- [[internal/rounds/rounds_test.go/TestRoundsAreOrderedByFileThenLineAndDeterministic]] (calls)
- [[internal/rounds/rounds_test.go/TestNoChangedRangeIsEmptyRounds]] (calls)
- [[internal/rounds/rounds_test.go/TestOwnerOfTwoNodesWithOneSpanIsTheSameInEveryOrder]] (calls)
- [[internal/scan/resolve_test.go/TestASuperCallLinksToTheSupertypesMethod]] (calls)

## Callers

- [[internal/rounds/fanout_test.go/fanoutGraph]] (calls)
- [[internal/rounds/helpers_test.go/hn]] (calls)
- [[internal/rounds/rounds_test.go/tn]] (calls)

## Callees

none
