---
id: "internal/graphbuild/build_test.go::repo"
source: "internal/graphbuild/build_test.go"
start: 12
end: 20
kind: "func"
is-test: false
tags: [rtdd/code]
---
# repo

#rtdd/code

## Tests

- [[internal/graphbuild/anonymous_spec_test.go/TestCallsInASpecsAnonymousCaseBodiesSelectTheSpec]] (calls)
- [[internal/graphbuild/build_test.go/TestBuildWithoutGraphifyIsTheScannersGraph]] (calls)
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
- [[internal/graphbuild/ignore_test.go/TestMaxStaleRatioOverride]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayDropsAnEdgeIntoARenamedNode]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildRescanMakesTheNamedFileStale]] (calls)
- [[internal/graphbuild/scanset_test.go/TestEveryChangedFileIsScannedWhateverItsExtension]] (calls)
- [[internal/graphbuild/scanset_test.go/TestNonCodeAndNewLanguageFilesDoNotCountTowardTheRatio]] (calls)
- [[internal/graphbuild/scanset_test.go/TestTooStaleCountsCodeFilesOnly]] (calls)
- [[internal/graphbuild/scanset_test.go/TestAFilteredChangedFileIsNeverScanned]] (calls)

## Callers

none

## Callees

- [[internal/gitctx/gittest/gittest.go/Init]] (calls)
- [[internal/gitctx/gittest/gittest.go/Write]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
