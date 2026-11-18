---
id: "internal/graph/config.go::LoadConfig"
source: "internal/graph/config.go"
start: 68
end: 111
kind: "func"
is-test: false
tags: [rtdd/code]
---
# LoadConfig

#rtdd/code

## Tests

- [[cmd/rtdd/init_migrate_test.go/TestInitMigratesAV02Repository]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (calls)
- [[internal/graph/classify_test.go/TestTestFilesOverrideReplacesTheDefaults]] (calls)
- [[internal/graph/config_test.go/TestLoadConfigWithoutAFileIsTheSpecDefaults]] (calls)
- [[internal/graph/config_test.go/TestLoadConfigKeyReplacesItsDefaultAndOtherKeysAreIgnored]] (calls)
- [[internal/graph/config_test.go/TestLoadConfigMovesProseOutOfAnOlderScanExclude]] (calls)
- [[internal/graph/config_test.go/TestLoadConfigRejectsAMalformedGlobAndRatio]] (calls)
- [[internal/graph/split_test.go/TestWholeFileTestsOverrideReplacesTheDefaults]] (calls)
- [[internal/graphbuild/ignore_test.go/TestMaxStaleRatioOverride]] (calls)
- [[internal/graphbuild/perf_test.go/TestWarmBuildOfThisRepositoryIsUnder1s]] (calls)
- [[internal/graphbuild/selfscan_test.go/TestThisRepositoryHasNoKeywordOrBuiltinNamedNodes]] (calls)
- [[internal/install/config_test.go/TestDefaultConfigLoadsAsTheGraphDefaults]] (calls)

## Callers

- [[cmd/rtdd/graph.go/cmdGraph]] (calls)
- [[cmd/rtdd/graphenv.go/buildGraph]] (calls)
- [[internal/rounds/perf_test.go/thisRepositoryGraph]] (calls)

## Callees

- [[internal/graph/config.go/DefaultConfig]] (calls)
- [[internal/paths/glob.go/ValidateGlob]] (calls)
