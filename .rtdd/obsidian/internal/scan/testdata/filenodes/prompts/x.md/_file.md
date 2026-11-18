---
id: "internal/scan/testdata/filenodes/prompts/x.md"
source: "internal/scan/testdata/filenodes/prompts/x.md"
start: 1
end: 10
kind: "file"
is-test: false
tags: [rtdd/file]
---
# x.md

#rtdd/file

## Tests

- [[cmd/rtdd/which_filenodes_test.go/TestWhichSelectsTheTestThatReadsAChangedFileWithNoDefinition]] (references)
- [[cmd/rtdd/which_filenodes_test.go/TestExplainAFileWithNoDefinition]] (references)
- [[internal/install/install_test.go/TestPlanAppendToRefusesAPathOutsideTheRepositoryOrInsideRtdd]] (references)
- [[internal/rounds/filenodes_test.go/TestAChangedFileWithNoDefinitionSelectsTheTestsThatNameIt]] (references)
- [[internal/rounds/filenodes_test.go/TestATestFilesTopLevelReferenceSelectsItsTests]] (references)
- [[internal/rounds/filenodes_test.go/TestLinksOfAFileListsATestFilesReferenceAsItsTests]] (references)
- [[internal/scan/refs_test.go/TestScanFileAttributesPathTokensToTheirOwner]] (references)
- [[internal/scan/refs_test.go/TestFileNodesFixtureReferences]] (references)

## Callers

- [[internal/scan/testdata/filenodes/tests/test_load.py/test_load]] (references)

## Callees

- [[internal/scan/testdata/filenodes/conf/app.yaml/_file]] (references)
