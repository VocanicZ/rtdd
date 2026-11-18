---
id: "internal/gitctx/gittest/gittest.go::Commit"
source: "internal/gitctx/gittest/gittest.go"
start: 104
end: 109
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Commit

#rtdd/code

## Tests

- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsTheGraphTestFilesAndOverLinkedNames]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsGraphifyFreshAndTooStale]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsLegacyFiles]] (calls)
- [[cmd/rtdd/explain_graph_test.go/TestExplainANameListsEveryDefinition]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitTwiceIgnoresTheGraphCacheExactlyOnce]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestWhichAfterAFreshInitNamesNothingUnderCursor]] (calls)
- [[cmd/rtdd/readme_example_test.go/TestREADMEWhichExampleIsRealOutput]] (calls)
- [[cmd/rtdd/which_comment_test.go/TestWhichIgnoresChangedLinesThatAreNotCode]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichNeverListsALogAsATest]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichWarnsWhenItSelectsMoreThanHalfTheSuite]] (calls)
- [[cmd/rtdd/which_rounds_test.go/TestWhichBaseAndExitCodes]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobIDsAreSafeForAnyPath]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobRunAndMergeExecuteTheSelection]] (calls)
- [[cmd/rtdd/which_wholefile_test.go/TestWhichWholeFileTestsListOnlyTheirPaths]] (calls)
- [[internal/gitctx/graphgit_test.go/TestDiffNamesSinceCoversCommittedStagedUnstagedAndDeleted]] (calls)
- [[internal/gitctx/graphgit_test.go/TestBlobIDsChangeOnlyWhenACommitChangesTheFile]] (calls)
- [[internal/gitctx/graphgit_test.go/TestCommitKnown]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheRescansOnlyChangedBlobsAndTheWorkingTreeChangedSet]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheRescansACommittedChange]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/perf_test.go/TestColdBuildOf10000FilesIsUnder10s]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsFourPassesOnTheRepoAsItStandsToday]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightLeavesTheRepositoryUntouched]] (calls)

## Callers

- [[cmd/rtdd/detect_test.go/newDetectableRepo]] (calls)
- [[cmd/rtdd/explain_graph_test.go/explainRepo]] (calls)
- [[cmd/rtdd/explain_graphify_test.go/explainGraphifyRepo]] (calls)
- [[cmd/rtdd/graph_test.go/graphRepo]] (calls)
- [[cmd/rtdd/init_gate_test.go/newUnsupportedRepo]] (calls)
- [[cmd/rtdd/init_migrate_test.go/v02Repository]] (calls)
- [[cmd/rtdd/init_next_test.go/newPolyglotRepo]] (calls)
- [[cmd/rtdd/main_test.go/newTestRepo]] (calls)
- [[cmd/rtdd/prose_test.go/proseRepo]] (calls)
- [[cmd/rtdd/which_e2e_test.go/e2eRepo]] (calls)
- [[cmd/rtdd/which_fanout_test.go/fanoutRepo]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/fileNodesRepo]] (calls)
- [[cmd/rtdd/which_graphify_base_test.go/graphifyBaseRepo]] (calls)
- [[cmd/rtdd/which_json_test.go/goldenWhichRepo]] (calls)
- [[cmd/rtdd/which_rounds_test.go/whichRepo]] (calls)
- [[internal/graphbuild/build_test.go/repo]] (calls)

## Callees

- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
- [[internal/gitctx/gittest/gittest.go/HeadShort]] (calls)
