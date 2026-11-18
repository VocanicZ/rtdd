---
id: "internal/gitctx/gittest/gittest.go::Write"
source: "internal/gitctx/gittest/gittest.go"
start: 92
end: 101
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Write

#rtdd/code

## Tests

- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsTheGraphTestFilesAndOverLinkedNames]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsGraphifyFreshAndTooStale]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsLegacyFiles]] (calls)
- [[cmd/rtdd/explain_graph_test.go/TestExplainANameListsEveryDefinition]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandReportsGraphifyAndStaleness]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandStaleFilesCountsEveryOverlaidFile]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandSaysWhyGraphifyWasIgnored]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitTwiceIgnoresTheGraphCacheExactlyOnce]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestWhichAfterAFreshInitNamesNothingUnderCursor]] (calls)
- [[cmd/rtdd/obsidian_test.go/TestObsidianNotePathsAreSafeAndUnique/TestObsidianRerunRewritesOnlyChangedNotes]] (calls)
- [[cmd/rtdd/readme_example_test.go/TestREADMEWhichExampleIsRealOutput]] (calls)
- [[cmd/rtdd/which_comment_test.go/TestWhichIgnoresChangedLinesThatAreNotCode]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichAChangelogAppendSelectsNothing]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichAChangedDocSelectsOnlyTheTestThatReadsIt]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichNeverListsALogAsATest]] (calls)
- [[cmd/rtdd/which_fanout_test.go/TestWhichWarnsWhenItSelectsMoreThanHalfTheSuite]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/TestWhichSelectsTheTestThatReadsAChangedFileWithNoDefinition]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/TestWhichListsAChangedFileNothingNamesAsUntested]] (calls)
- [[cmd/rtdd/which_rounds_test.go/TestWhichWithEmptyRoundsSaysNoLinkedTestAndExits0]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobIDsAreSafeForAnyPath]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobRunAndMergeExecuteTheSelection]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobWithEmptyRoundsWritesNothing]] (calls)
- [[internal/gitctx/graphgit_test.go/TestDiffNamesSinceCoversCommittedStagedUnstagedAndDeleted]] (calls)
- [[internal/gitctx/graphgit_test.go/TestBlobIDsChangeOnlyWhenACommitChangesTheFile]] (calls)
- [[internal/gitctx/graphgit_test.go/TestCommitKnown]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheRescansOnlyChangedBlobsAndTheWorkingTreeChangedSet]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheRescansACommittedChange]] (calls)
- [[internal/graphbuild/cache_test.go/TestRescanOfAnUnchangedEditDoesNotRewriteTheCache]] (calls)
- [[internal/graphbuild/cache_test.go/TestCachedCallerOfARenamedFunctionIsNotLeftDangling]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheServesGraphifysCommittedStaleFiles]] (calls)
- [[internal/graphbuild/ignore_test.go/TestMaxStaleRatioOverride]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayDropsAnEdgeIntoARenamedNode]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/scanset_test.go/TestEveryChangedFileIsScannedWhateverItsExtension]] (calls)
- [[internal/graphbuild/scanset_test.go/TestNonCodeAndNewLanguageFilesDoNotCountTowardTheRatio]] (calls)
- [[internal/graphbuild/scanset_test.go/TestTooStaleCountsCodeFilesOnly]] (calls)
- [[internal/graphbuild/scanset_test.go/TestAFilteredChangedFileIsNeverScanned]] (calls)

## Callers

- [[cmd/rtdd/detect_test.go/newDetectableRepo]] (calls)
- [[cmd/rtdd/explain_graph_test.go/explainRepo]] (calls)
- [[cmd/rtdd/explain_graphify_test.go/explainGraphifyRepo]] (calls)
- [[cmd/rtdd/graph_test.go/graphRepo]] (calls)
- [[cmd/rtdd/graph_test.go/writeGraphifyFor]] (calls)
- [[cmd/rtdd/init_gate_test.go/newUnsupportedRepo]] (calls)
- [[cmd/rtdd/init_next_test.go/newPolyglotRepo]] (calls)
- [[cmd/rtdd/main_test.go/writeFile]] (calls)
- [[cmd/rtdd/main_test.go/newTestRepo]] (calls)
- [[cmd/rtdd/prose_test.go/proseRepo]] (calls)
- [[cmd/rtdd/uninstall_test.go/initialisedRepo]] (calls)
- [[cmd/rtdd/uninstall_test.go/legacyRepo]] (calls)
- [[cmd/rtdd/which_e2e_test.go/e2eRepo]] (calls)
- [[cmd/rtdd/which_e2e_test.go/e2eEdit]] (calls)
- [[cmd/rtdd/which_fanout_test.go/fanoutRepo]] (calls)
- [[cmd/rtdd/which_graphify_base_test.go/graphifyBaseRepo]] (calls)
- [[cmd/rtdd/which_json_test.go/copyTree]] (calls)
- [[cmd/rtdd/which_rounds_test.go/whichRepo]] (calls)
- [[cmd/rtdd/which_rounds_test.go/editTotalAndUnused]] (calls)
- [[internal/graphbuild/build_test.go/repo]] (calls)
- [[internal/graphbuild/ignore_test.go/edit]] (calls)
- [[internal/graphbuild/overlay_test.go/writeGraphify]] (calls)

## Callees

none
