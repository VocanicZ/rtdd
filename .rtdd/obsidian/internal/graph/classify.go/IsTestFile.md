---
id: "internal/graph/classify.go::IsTestFile"
source: "internal/graph/classify.go"
start: 24
end: 29
kind: "func"
is-test: false
tags: [rtdd/code]
---
# IsTestFile

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestInterfaceContractDocumentsTheFullEscalateExclusion]] (calls)
- [[internal/graph/datafiles_test.go/TestDataAndLogFilesAreNeverTestFiles]] (calls)
- [[internal/graph/split_test.go/TestWholeFileTestsFileWithNoNodeGetsItsFileNode]] (calls)

## Callers

- [[cmd/rtdd/doctor.go/renderDoctor]] (calls)
- [[internal/graph/classify.go/Classify]] (calls)
- [[internal/graph/classify.go/SplitTests]] (calls)

## Callees

- [[internal/graph/classify.go/isDataFile]] (calls)
- [[internal/graph/classify.go/matchAny]] (calls)
