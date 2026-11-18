---
id: "internal/graph/config.go::DefaultConfig"
source: "internal/graph/config.go"
start: 33
end: 45
kind: "func"
is-test: false
tags: [rtdd/code]
---
# DefaultConfig

#rtdd/code

## Tests

- [[cmd/rtdd/init_migrate_test.go/TestInitMigratesAV02Repository]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (calls)
- [[internal/contract/plan_n3_test.go/TestN3PlanSettlesTheSharedDecisions]] (calls)
- [[internal/graph/classify_test.go/TestEachDefaultTestFilesGlobMarksAFuncNodeATest]] (calls)
- [[internal/graph/classify_test.go/TestClassesAndNonTestFilesAreNeverTests]] (calls)
- [[internal/graph/config_test.go/TestLoadConfigMovesProseOutOfAnOlderScanExclude]] (calls)
- [[internal/graph/datafiles_test.go/TestDataAndLogFilesAreNeverTestFiles]] (calls)
- [[internal/graph/split_test.go/TestWholeFileTestsGlobMakesEveryFuncAHelperUnderOneFileNode]] (calls)
- [[internal/graph/split_test.go/TestATestFileWithNoTestIsWholeFileAndOtherFilesKeepTheirTests]] (calls)
- [[internal/graph/split_test.go/TestWholeFileTestsFileWithNoNodeGetsItsFileNode]] (calls)
- [[internal/graphbuild/build_test.go/TestBuildWithoutGraphifyIsTheScannersGraph]] (calls)
- [[internal/graphbuild/cache_test.go/TestRescanOfAnUnchangedEditDoesNotRewriteTheCache]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildRescanMakesTheNamedFileStale]] (calls)
- [[internal/graphbuild/perf_test.go/TestColdBuildOf10000FilesIsUnder10s]] (calls)
- [[internal/install/config_test.go/TestDefaultConfigLoadsAsTheGraphDefaults]] (calls)
- [[internal/install/config_test.go/TestDefaultConfigNamesEveryKeyAndNoV02Key]] (calls)
- [[internal/scan/filter_test.go/TestFilterDropsBinaryOversizedAndEveryDefaultExclude]] (calls)

## Callers

- [[internal/graph/config.go/LoadConfig]] (calls)
- [[internal/graphbuild/build_test.go/build]] (calls)
- [[internal/install/config.go/DefaultConfig]] (calls)

## Callees

- [[docs/results/worked-example-v3/graph.json/_file]] (references)
- [[internal/graphify/testdata/graphify-out/graph.json/_file]] (references)
