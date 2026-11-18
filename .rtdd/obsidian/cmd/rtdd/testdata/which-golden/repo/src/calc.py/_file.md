---
id: "cmd/rtdd/testdata/which-golden/repo/src/calc.py"
source: "cmd/rtdd/testdata/which-golden/repo/src/calc.py"
start: 1
end: 10
kind: "file"
is-test: false
tags: [rtdd/file]
---
# calc.py

#rtdd/file

## Tests

- [[bench/tests/test_chart.py/test_the_rounds_figure_refuses_a_round_the_graph_does_not_support]] (references)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsGraphifyFreshAndTooStale]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainPrintsTestsCallersAndCallees]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainPrintsTestsCallersAndCallees]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainResolvesAFileRelativeToTheWorkingDirectory]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainResolvesAFileRelativeToTheWorkingDirectory]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainANameListsEveryDefinition]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainANameListsEveryDefinition]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainExitCodes]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainExitCodes]] (references)
- [[cmd/rtdd/explain_graph_test.go/TestExplainImportsNoCoveragePipelinePackage]] (references)
- [[cmd/rtdd/explain_graphify_test.go/TestExplainFileLineRescansTheFileUnderGraphify]] (references)
- [[cmd/rtdd/explain_graphify_test.go/TestExplainNameAndFileStillReadGraphify]] (references)
- [[cmd/rtdd/graph_test.go/TestGraphCommandSaysWhyGraphifyWasIgnored]] (references)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (references)
- [[cmd/rtdd/obsidian_test.go/TestObsidianNoOpenWritesOneNotePerNode]] (references)
- [[cmd/rtdd/obsidian_test.go/TestObsidianNotePathsAreSafeAndUnique/TestObsidianRerunRewritesOnlyChangedNotes]] (references)
- [[cmd/rtdd/prose_test.go/TestGraphNeverScansProseFilesAsCode]] (references)
- [[cmd/rtdd/readme_example_test.go/TestREADMEWhichExampleIsRealOutput]] (references)
- [[cmd/rtdd/which_graphify_base_test.go/TestWhichBaseRescansFilesChangedAgainstTheBaseUnderGraphify]] (references)
- [[cmd/rtdd/which_rounds_test.go/TestWhichPrintsChangedNodesRoundsUntestedAndGraphSource]] (references)
- [[cmd/rtdd/which_rounds_test.go/TestWhichWithEmptyRoundsSaysNoLinkedTestAndExits0]] (references)
- [[cmd/rtdd/which_rounds_test.go/TestWhichBaseAndExitCodes]] (references)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobIDsAreSafeForAnyPath]] (references)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobRunAndMergeExecuteTheSelection]] (references)
- [[cmd/rtdd/which_swarm_test.go/TestWhichSwarmJobWithEmptyRoundsWritesNothing]] (references)
- [[internal/graphbuild/build_test.go/TestBuildWithoutGraphifyIsTheScannersGraph]] (references)
- [[internal/graphbuild/build_test.go/TestBuildWithoutGraphifyIsTheScannersGraph]] (references)
- [[internal/graphbuild/cache_test.go/TestCacheRescansOnlyChangedBlobsAndTheWorkingTreeChangedSet]] (references)
- [[internal/graphbuild/cache_test.go/TestRescanOfAnUnchangedEditDoesNotRewriteTheCache]] (references)
- [[internal/graphbuild/cache_test.go/TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs]] (references)
- [[internal/graphbuild/cache_test.go/TestCachedCallerOfARenamedFunctionIsNotLeftDangling]] (references)
- [[internal/graphbuild/cache_test.go/TestCacheServesGraphifysCommittedStaleFiles]] (references)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (references)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (references)
- [[internal/graphbuild/overlay_test.go/TestOverlayDropsAnEdgeIntoARenamedNode]] (references)
- [[internal/graphbuild/overlay_test.go/TestOverlayDropsAnEdgeIntoARenamedNode]] (references)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (references)
- [[internal/graphbuild/overlay_test.go/TestOverlayLinksAStaleFilesCallsToItsOwnFileTypeFirst]] (references)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (references)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (references)
- [[internal/graphbuild/overlay_test.go/TestBuildRescanMakesTheNamedFileStale]] (references)
- [[internal/graphbuild/overlay_test.go/TestBuildRescanMakesTheNamedFileStale]] (references)
- [[internal/graphify/load_test.go/TestLoadKeepsCodeNodesAndDropsNonCodeRelations]] (references)
- [[internal/rounds/helpers_test.go/TestHelperIsTransparentToTheTestsThatCallIt]] (references)
- [[internal/rounds/helpers_test.go/TestHelperNoTestCallsSelectsItsWholeFile]] (references)
- [[internal/rounds/links_test.go/TestLinksOfSplitsTestsCallersAndCallees]] (references)
- [[internal/rounds/links_test.go/TestLinkerMatchesLinksOf]] (references)
- [[internal/scan/fallback_test.go/TestSameDirectoryIsLocalOnlyInPackageScopedLanguages]] (references)

## Callers

- [[README.md/_file]] (references)
- [[cmd/rtdd/explain_graph_test.go/explainRepo]] (references)
- [[cmd/rtdd/explain_graphify_test.go/explainGraphifyRepo]] (references)
- [[cmd/rtdd/graph_test.go/graphRepo]] (references)
- [[cmd/rtdd/graph_test.go/writeGraphifyFor]] (references)
- [[cmd/rtdd/init_migrate_test.go/v02Repository]] (references)
- [[cmd/rtdd/prose_test.go/proseRepo]] (references)
- [[cmd/rtdd/testdata/which-golden/want.json/_file]] (references)
- [[cmd/rtdd/which_graphify_base_test.go/graphifyBaseRepo]] (references)
- [[cmd/rtdd/which_rounds_test.go/whichRepo]] (references)
- [[cmd/rtdd/which_rounds_test.go/editTotalAndUnused]] (references)
- [[docs/outcomes/README.positive.md/_file]] (references)
- [[docs/plans/06-m6d-shipped-adapters.md/_file]] (references)
- [[docs/plans/09-node-graph-core.md/_file]] (references)
- [[docs/plans/10-rounds-cutover.md/_file]] (references)
- [[docs/plans/11-skill-install.md/_file]] (references)
- [[docs/reference.md/_file]] (references)
- [[docs/results/worked-example-v3/graph.json/_file]] (references)
- [[docs/results/worked-example-v3/repo.json/_file]] (references)
- [[docs/results/worked-example-v3/which-add.json/_file]] (references)
- [[internal/graphbuild/overlay_test.go/overlayGraphify]] (references)
- [[internal/graphify/testdata/graphify-out/graph.json/_file]] (references)
- [[internal/graphify/testdata/graphify-out/manifest.json/_file]] (references)
- [[internal/scan/testdata/python/want.json/_file]] (references)

## Callees

none
