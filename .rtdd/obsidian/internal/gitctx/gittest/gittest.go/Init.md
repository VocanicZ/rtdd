---
id: "internal/gitctx/gittest/gittest.go::Init"
source: "internal/gitctx/gittest/gittest.go"
start: 74
end: 89
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Init

#rtdd/code

## Tests

- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsTheGraphTestFilesAndOverLinkedNames]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestWhichAfterAFreshInitNamesNothingUnderCursor]] (calls)
- [[cmd/rtdd/readme_example_test.go/TestREADMEWhichExampleIsRealOutput]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallFromHomeKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesFromHomeKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/which_comment_test.go/TestWhichIgnoresChangedLinesThatAreNotCode]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichWarnsWhenItSelectsMoreThanHalfTheSuite]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobIDsAreSafeForAnyPath]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobRunAndMergeExecuteTheSelection]] (calls)
- [[cmd/rtdd/which_wholefile_test.go/TestWhichWholeFileTestsListOnlyTheirPaths]] (calls)
- [[internal/gitctx/gittest/gittest_test.go/TestInitRepoNeverRunsGitGCBehindTheTest]] (calls)
- [[internal/gitctx/graphgit_test.go/TestDiffNamesSinceCoversCommittedStagedUnstagedAndDeleted]] (calls)
- [[internal/gitctx/graphgit_test.go/TestBlobIDsChangeOnlyWhenACommitChangesTheFile]] (calls)
- [[internal/gitctx/graphgit_test.go/TestBlobIDsOnAnUnbornHeadIsEmpty]] (calls)
- [[internal/gitctx/graphgit_test.go/TestCommitKnown]] (calls)
- [[internal/gitctx/reporoot_test.go/TestFindRepoRootAgreesWithGitOnARealRepository]] (calls)
- [[internal/graphbuild/perf_test.go/TestColdBuildOf10000FilesIsUnder10s]] (calls)

## Callers

- [[cmd/rtdd/detect_test.go/newDetectableRepo]] (calls)
- [[cmd/rtdd/explain_graph_test.go/explainRepo]] (calls)
- [[cmd/rtdd/explain_graphify_test.go/explainGraphifyRepo]] (calls)
- [[cmd/rtdd/graph_test.go/graphRepo]] (calls)
- [[cmd/rtdd/init_gate_test.go/newUnsupportedRepo]] (calls)
- [[cmd/rtdd/init_next_test.go/newPolyglotRepo]] (calls)
- [[cmd/rtdd/main_test.go/newTestRepo]] (calls)
- [[cmd/rtdd/prose_test.go/proseRepo]] (calls)
- [[cmd/rtdd/uninstall_test.go/initialisedRepo]] (calls)
- [[cmd/rtdd/uninstall_test.go/legacyRepo]] (calls)
- [[cmd/rtdd/which_e2e_test.go/e2eRepo]] (calls)
- [[cmd/rtdd/which_fanout_test.go/fanoutRepo]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/fileNodesRepo]] (calls)
- [[cmd/rtdd/which_graphify_base_test.go/graphifyBaseRepo]] (calls)
- [[cmd/rtdd/which_json_test.go/goldenWhichRepo]] (calls)
- [[cmd/rtdd/which_rounds_test.go/whichRepo]] (calls)
- [[internal/graphbuild/build_test.go/repo]] (calls)

## Callees

- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
