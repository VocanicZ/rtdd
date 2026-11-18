---
id: "internal/graphbuild/build.go::Build"
source: "internal/graphbuild/build.go"
start: 65
end: 129
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Build

#rtdd/code

## Tests

- [[internal/graphbuild/build_test.go/TestBuildWithoutGraphifyIsTheScannersGraph]] (calls)
- [[internal/graphbuild/ignore_test.go/TestMaxStaleRatioOverride]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildRescanMakesTheNamedFileStale]] (calls)
- [[internal/graphbuild/perf_test.go/TestColdBuildOf10000FilesIsUnder10s]] (calls)
- [[internal/graphbuild/perf_test.go/TestWarmBuildOfThisRepositoryIsUnder1s]] (calls)
- [[internal/graphbuild/selfscan_test.go/TestThisRepositoryHasNoKeywordOrBuiltinNamedNodes]] (calls)

## Callers

- [[cmd/rtdd/graph.go/cmdGraph]] (calls)
- [[cmd/rtdd/graphenv.go/buildGraph]] (calls)
- [[internal/graphbuild/build_test.go/build]] (calls)
- [[internal/rounds/perf_test.go/thisRepositoryGraph]] (calls)

## Callees

- [[docs/results/worked-example-v3/graph.json/_file]] (references)
- [[internal/gitctx/gitctx.go/HeadSHA]] (calls)
- [[internal/gitctx/graphgit.go/BlobIDs]] (calls)
- [[internal/gitctx/listfiles.go/ListFiles]] (calls)
- [[internal/graph/classify.go/Classify]] (calls)
- [[internal/graph/classify.go/SplitTests]] (calls)
- [[internal/graphbuild/build.go/useGraphify]] (calls)
- [[internal/graphbuild/build.go/changedSet]] (calls)
- [[internal/graphbuild/build.go/currentChangedSet]] (calls)
- [[internal/graphbuild/build.go/addFiles]] (calls)
- [[internal/graphbuild/build.go/matchAny]] (calls)
- [[internal/graphbuild/cache.go/scannerID]] (calls)
- [[internal/graphbuild/cache.go/readCache]] (calls)
- [[internal/graphbuild/cache.go/scanCached]] (calls)
- [[internal/graphbuild/cache.go/writeCache]] (calls)
- [[internal/graphbuild/cache.go/current]] (calls)
- [[internal/graphbuild/overlay.go/overlay_2]] (calls)
- [[internal/graphify/load.go/Load]] (calls)
- [[internal/graphify/testdata/graphify-out/graph.json/_file]] (references)
- [[internal/scan/filter.go/Filter]] (calls)
- [[internal/scan/scan.go/AssembleStats]] (calls)
