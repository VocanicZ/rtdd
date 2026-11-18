---
id: "internal/scan/testdata/filenodes/conf/app.yaml"
source: "internal/scan/testdata/filenodes/conf/app.yaml"
start: 1
end: 2
kind: "file"
is-test: false
tags: [rtdd/file]
---
# app.yaml

#rtdd/file

## Tests

- [[cmd/rtdd/which_filenodes_test.go/TestWhichSelectsTheTestThatReadsAChangedFileWithNoDefinition]] (references)
- [[cmd/rtdd/which_filenodes_test.go/TestExplainAFileWithNoDefinition]] (references)
- [[internal/rounds/filenodes_test.go/TestTheTestsOfWhatLoadsAChangedFileAreRoundOne]] (references)
- [[internal/scan/refs_test.go/TestScanProseReadsPathsAndNoDefinition]] (references)
- [[internal/scan/refs_test.go/TestFileNodesFixtureReferences]] (references)

## Callers

- [[docs/plans/02-m1b-python-adapter.md/_file]] (references)
- [[docs/plans/03-m2-signal.md/_file]] (references)
- [[internal/scan/testdata/filenodes/prompts/x.md/_file]] (references)
- [[internal/scan/testdata/filenodes/tests/test_load.py/test_load]] (references)

## Callees

none
