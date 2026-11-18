---
id: "internal/rounds/rounds.go::Rounds"
source: "internal/rounds/rounds.go"
start: 125
end: 290
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Rounds

#rtdd/code

## Tests

- [[cmd/rtdd/event_test.go/TestEventLogWhichWritesOneLineWithTheJSONCounts]] (calls)
- [[cmd/rtdd/which_graphify_base_test.go/TestWhichBaseRescansFilesChangedAgainstTheBaseUnderGraphify]] (calls)
- [[cmd/rtdd/which_json_test.go/TestWhichJSONRoundFilesAreTheDeduplicatedTestFiles]] (calls)
- [[internal/contract/plan_n2_test.go/TestN2PlanSettlesTheSharedDecisions]] (calls)
- [[internal/graphbuild/anonymous_spec_test.go/TestCallsInASpecsAnonymousCaseBodiesSelectTheSpec]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedProseFileAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedProseFileATestReadsIsRound1Only]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedDataFileAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedTestAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestAChangedWholeFileTestAddsNothingToRound2]] (calls)
- [[internal/rounds/fanout_test.go/TestRound2OfCodeAndTestChangedTogetherComesFromTheCode]] (calls)
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
- [[internal/rounds/perf_test.go/TestRoundsOfEveryLineOfThisRepositoryIsUnder1s]] (calls)
- [[internal/rounds/perf_test.go/BenchmarkRoundsOnThisRepository]] (calls)
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
- [[internal/rounds/rounds_test.go/TestNoChangedRangeIsEmptyRounds]] (calls)
- [[internal/rounds/rounds_test.go/TestOwnerOfTwoNodesWithOneSpanIsTheSameInEveryOrder]] (calls)
- [[internal/rounds/samelanguage_test.go/TestAOneLineGoEditSelectsNoPythonBenchTest]] (calls)

## Callers

- [[cmd/rtdd/which.go/cmdWhich]] (calls)
- [[cmd/rtdd/which_swarm_test.go/selectedFiles]] (calls)

## Callees

- [[internal/graph/classify.go/WholeFileNode]] (calls)
- [[internal/rounds/rounds.go/newIndex]] (calls)
- [[internal/rounds/rounds.go/owners]] (calls)
- [[internal/rounds/rounds.go/spreadsNothing]] (calls)
- [[internal/rounds/rounds.go/runnable]] (calls)
- [[internal/rounds/rounds.go/ordered]] (calls)
