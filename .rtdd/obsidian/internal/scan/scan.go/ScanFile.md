---
id: "internal/scan/scan.go::ScanFile"
source: "internal/scan/scan.go"
start: 66
end: 288
kind: "func"
is-test: false
tags: [rtdd/code]
---
# ScanFile

#rtdd/code

## Tests

- [[internal/graphbuild/overlay_test.go/TestOverlayLinksAStaleFilesCallsToItsOwnFileTypeFirst]] (calls)
- [[internal/scan/refs_test.go/TestScanFileAttributesPathTokensToTheirOwner]] (calls)
- [[internal/scan/resolve_test.go/TestImportSpecsResolveToDirectoriesAndStayInsideTheRepository]] (calls)
- [[internal/scan/resolve_test.go/TestRefsOfAnAbsentNodeLinkNothing]] (calls)
- [[internal/scan/rules_test.go/TestInnermostNodeOwnsALineAndAClassHasMethodEdges]] (calls)
- [[internal/scan/rules_test.go/TestEverySameNamedDefinitionIsCalledButNotFromTheDefinitionLine]] (calls)
- [[internal/scan/rules_test.go/TestAQualifiedNameIsACallEvenWhenItIsAKeyword]] (calls)
- [[internal/scan/rules_test.go/TestAWrappedHeaderThatNeverClosesDefinesNothing]] (calls)
- [[internal/scan/rules_test.go/TestAnOpenerTheFileNeverClosesBlanksOnlyItself]] (calls)
- [[internal/scan/scanfiles_test.go/TestScanFilesKeepsInputOrderAndSkipsUnreadableFiles]] (calls)
- [[internal/scan/shapes_t3_test.go/TestRubyBlockTestsAndCommandCalls]] (calls)
- [[internal/scan/shapes_t3_test.go/TestShellCommandFormCalls]] (calls)
- [[internal/scan/shapes_t3_test.go/TestCommandFormSkipsDeclarationsAndLiterals]] (calls)

## Callers

- [[internal/scan/resolve_test.go/assembleAll]] (calls)
- [[internal/scan/rules_test.go/linkAll]] (calls)
- [[internal/scan/scan.go/ScanFilesProse]] (calls)
- [[internal/scan/shapes_test.go/spans]] (calls)

## Callees

- [[internal/scan/patterns.go/matchDefinition]] (calls)
- [[internal/scan/patterns.go/declarations]] (calls)
- [[internal/scan/patterns.go/supertypes]] (calls)
- [[internal/scan/patterns.go/imports]] (calls)
- [[internal/scan/refs.go/pathTokens]] (calls)
- [[internal/scan/refs.go/sortedSet]] (calls)
- [[internal/scan/scan.go/splitLines]] (calls)
- [[internal/scan/scan.go/receiverOf]] (calls)
- [[internal/scan/scan.go/isWord]] (calls)
- [[internal/scan/scan.go/indentOf]] (calls)
- [[internal/scan/scan.go/header]] (calls)
- [[internal/scan/scan.go/endLine]] (calls)
- [[internal/scan/scan.go/code]] (calls)
