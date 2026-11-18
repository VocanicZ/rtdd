---
id: "internal/graph/classify.go::Classify"
source: "internal/graph/classify.go"
start: 43
end: 54
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Classify

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestInterfaceContractCarriesEveryM2Addition]] (calls)
- [[internal/graph/classify_test.go/TestEachDefaultTestFilesGlobMarksAFuncNodeATest]] (calls)
- [[internal/graph/classify_test.go/TestClassesAndNonTestFilesAreNeverTests]] (calls)
- [[internal/graph/classify_test.go/TestTestFilesOverrideReplacesTheDefaults]] (calls)
- [[internal/graph/datafiles_test.go/TestDataAndLogFilesAreNeverTestFiles]] (calls)

## Callers

- [[internal/graph/split_test.go/split]] (calls)
- [[internal/graphbuild/build.go/Build]] (calls)

## Callees

- [[internal/graph/classify.go/IsTestFile]] (calls)
