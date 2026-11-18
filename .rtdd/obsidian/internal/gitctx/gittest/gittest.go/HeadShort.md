---
id: "internal/gitctx/gittest/gittest.go::HeadShort"
source: "internal/gitctx/gittest/gittest.go"
start: 112
end: 115
kind: "func"
is-test: false
tags: [rtdd/code]
---
# HeadShort

#rtdd/code

## Tests

- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsTheGraphTestFilesAndOverLinkedNames]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsGraphifyFreshAndTooStale]] (calls)
- [[cmd/rtdd/event_test.go/TestEventLogWhichWritesOneLineWithTheJSONCounts]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandPrintsSourceCountsAndStaleness]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandJSONShape]] (calls)
- [[cmd/rtdd/which_json_test.go/TestWhichJSONGolden]] (calls)
- [[cmd/rtdd/which_rounds_test.go/TestWhichPrintsChangedNodesRoundsUntestedAndGraphSource]] (calls)
- [[cmd/rtdd/which_rounds_test.go/TestWhichBaseAndExitCodes]] (calls)
- [[internal/gitctx/gittest/gittest_test.go/TestInitRepoCommitsTheTree]] (calls)
- [[internal/graphbuild/build_test.go/TestBuildWithoutGraphifyIsTheScannersGraph]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs]] (calls)

## Callers

- [[cmd/rtdd/main_test.go/headShort]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
- [[internal/installtest/release_preflight_test.go/buildFixtureRepo]] (calls)

## Callees

- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
