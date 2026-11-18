---
id: "internal/gitctx/gittest/gittest.go::Run"
source: "internal/gitctx/gittest/gittest.go"
start: 22
end: 29
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Run

#rtdd/code

## Tests

- [[cmd/rtdd/graph_test.go/TestGraphCommandReportsGraphifyAndStaleness]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandStaleFilesCountsEveryOverlaidFile]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandSaysWhyGraphifyWasIgnored]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (calls)
- [[cmd/rtdd/init_test.go/TestInitProjectSkillFromHomeKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/main_test.go/TestNothingExitsOne]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallFromHomeKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesFromHomeKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallWithNoHomeVariableKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallWithARelativeHomeKeepsTheUserLevelInstalls]] (calls)
- [[cmd/rtdd/which_filenodes_test.go/TestWhichSelectsTheTestThatReadsAChangedFileWithNoDefinition]] (calls)
- [[cmd/rtdd/which_perf_test.go/TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s]] (calls)
- [[internal/contract/contract_test.go/TestInterfaceContractRecordsInitrepo]] (calls)
- [[internal/contract/contract_test.go/TestInterfaceContractCarriesEveryM2Addition]] (calls)
- [[internal/gitctx/gittest/gittest_test.go/TestInitRepoCommitsTheTree]] (calls)
- [[internal/gitctx/gittest/gittest_test.go/TestInitRepoNeverRunsGitGCBehindTheTest]] (calls)
- [[internal/gitctx/graphgit_test.go/TestDiffNamesSinceCoversCommittedStagedUnstagedAndDeleted]] (calls)
- [[internal/gitctx/graphgit_test.go/TestCommitKnown]] (calls)
- [[internal/gitctx/reporoot_test.go/TestIsRepoRoot]] (calls)
- [[internal/graphbuild/cache_test.go/TestCorruptOrForeignCacheIsRebuiltNotAnError]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheServesGraphifysCommittedStaleFiles]] (calls)
- [[internal/graphbuild/ignore_test.go/TestGraphifyIsIgnoredWithAReason]] (calls)
- [[internal/graphbuild/ignore_test.go/TestMaxStaleRatioOverride]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayLinksAStaleFilesCallsToItsOwnFileTypeFirst]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsPlaceholdersAndMissingUvExplicitly]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightLeavesTheRepositoryUntouched]] (calls)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightOnThisRepoReportsFourPasses]] (calls)
- [[internal/scan/rules_test.go/TestACallLinksToSameNamedDefinitionsOfItsOwnFileTypeFirst]] (calls)

## Callers

- [[cmd/rtdd/explain_graphify_test.go/explainGraphifyRepo]] (calls)
- [[cmd/rtdd/main_test.go/gitRun]] (calls)
- [[cmd/rtdd/which_graphify_base_test.go/graphifyBaseRepo]] (calls)
- [[internal/gitctx/gittest/gittest.go/Init]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
- [[internal/gitctx/gittest/gittest.go/HeadShort]] (calls)
- [[internal/graphbuild/overlay_test.go/fullHead]] (calls)
- [[internal/installtest/release_preflight_test.go/buildFixtureRepo]] (calls)
- [[internal/installtest/release_preflight_test.go/runPreflight]] (calls)

## Callees

- [[internal/gitctx/gittest/gittest.go/run_2]] (calls)
