---
id: "cmd/rtdd/graph_test.go::graphRepo"
source: "cmd/rtdd/graph_test.go"
start: 14
end: 21
kind: "func"
is-test: false
tags: [rtdd/code]
---
# graphRepo

#rtdd/code

## Tests

- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsGraphifyFreshAndTooStale]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorExitCodes]] (calls)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsLegacyFiles]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandPrintsSourceCountsAndStaleness]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandJSONShape]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandExitCodes]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandReportsGraphifyAndStaleness]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandStaleFilesCountsEveryOverlaidFile]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandSaysWhyGraphifyWasIgnored]] (calls)
- [[cmd/rtdd/graph_test.go/TestGraphCommandNeverRunsGraphify]] (calls)
- [[cmd/rtdd/init_migrate_test.go/TestInitOnAFreshRepositoryPrintsNoRemoval]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitTwiceIgnoresTheGraphCacheExactlyOnce]] (calls)
- [[cmd/rtdd/init_shapes_test.go/TestInitHasNoForceFlag]] (calls)
- [[cmd/rtdd/removed_test.go/TestRemovedCommandsExit2NamingV030]] (calls)
- [[cmd/rtdd/removed_test.go/TestHelpListsNoRemovedCommand]] (calls)

## Callers

- [[cmd/rtdd/init_migrate_test.go/v02Repository]] (calls)

## Callees

- [[cmd/rtdd/testdata/which-golden/edit/src/calc.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/repo/src/calc.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/repo/tests/test_calc.py/_file]] (references)
- [[internal/gitctx/gittest/gittest.go/Init]] (calls)
- [[internal/gitctx/gittest/gittest.go/Write]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
- [[internal/scan/testdata/python/calc.py/_file]] (references)
- [[internal/scan/testdata/python/test_calc.py/_file]] (references)
