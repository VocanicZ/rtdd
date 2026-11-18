---
id: "internal/graphbuild/build_test.go::build"
source: "internal/graphbuild/build_test.go"
start: 23
end: 30
kind: "func"
is-test: false
tags: [rtdd/code]
---
# build

#rtdd/code

## Tests

- [[internal/gitctx/reporoot_test.go/TestIsRepoRoot]] (calls)
- [[internal/graphbuild/anonymous_spec_test.go/TestCallsInASpecsAnonymousCaseBodiesSelectTheSpec]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheRescansOnlyChangedBlobsAndTheWorkingTreeChangedSet]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheRescansACommittedChange]] (calls)
- [[internal/graphbuild/cache_test.go/TestWarmBuildEqualsColdBuild]] (calls)
- [[internal/graphbuild/cache_test.go/TestWarmBuildKeepsMethodEdges]] (calls)
- [[internal/graphbuild/cache_test.go/TestRescanOfAnUnchangedEditDoesNotRewriteTheCache]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs]] (calls)
- [[internal/graphbuild/cache_test.go/TestCorruptOrForeignCacheIsRebuiltNotAnError]] (calls)
- [[internal/graphbuild/cache_test.go/TestCachedCallerOfARenamedFunctionIsNotLeftDangling]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheServesGraphifysCommittedStaleFiles]] (calls)
- [[internal/graphbuild/cache_test.go/TestWarmBuildKeepsImportsAndSupertypes]] (calls)
- [[internal/graphbuild/ignore_test.go/TestGraphifyIsIgnoredWithAReason]] (calls)
- [[internal/graphbuild/ignore_test.go/TestTooStaleKeepsTheStaleSetItMeasured]] (calls)
- [[internal/graphbuild/ignore_test.go/TestExactlyHalfStaleKeepsGraphify]] (calls)
- [[internal/graphbuild/ignore_test.go/TestGraphifyWithNoCodeFilesIsTooStale]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayDropsAnEdgeIntoARenamedNode]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/scanset_test.go/TestEveryChangedFileIsScannedWhateverItsExtension]] (calls)
- [[internal/graphbuild/scanset_test.go/TestNonCodeAndNewLanguageFilesDoNotCountTowardTheRatio]] (calls)
- [[internal/graphbuild/scanset_test.go/TestTooStaleCountsCodeFilesOnly]] (calls)
- [[internal/graphbuild/scanset_test.go/TestAFilteredChangedFileIsNeverScanned]] (calls)
- [[internal/scan/fallback_test.go/TestLocalReceiverStaysLocal]] (calls)

## Callers

none

## Callees

- [[internal/graph/config.go/DefaultConfig]] (calls)
- [[internal/graphbuild/build.go/Build]] (calls)
