---
id: "internal/scan/resolve_test.go::assembleAll"
source: "internal/scan/resolve_test.go"
start: 13
end: 19
kind: "func"
is-test: false
tags: [rtdd/code]
---
# assembleAll

#rtdd/code

## Tests

- [[internal/scan/fallback_test.go/TestFallbackNeverCrossesLanguages]] (calls)
- [[internal/scan/fallback_test.go/TestExternalReceiverLinksNowhere]] (calls)
- [[internal/scan/fallback_test.go/TestReceiverNamingAFileLinksToThatFile]] (calls)
- [[internal/scan/fallback_test.go/TestLocalReceiverStaysLocal]] (calls)
- [[internal/scan/fallback_test.go/TestSameDirectoryIsLocalOnlyInPackageScopedLanguages]] (calls)
- [[internal/scan/resolve_test.go/TestACallInsideAClassLinksToItsOwnClassesMethod]] (calls)
- [[internal/scan/resolve_test.go/TestAReceiversDeclaredTypePicksItsClassesMethod]] (calls)
- [[internal/scan/resolve_test.go/TestACallPrefersTheDefinitionInAnImportedFile]] (calls)
- [[internal/scan/resolve_test.go/TestACallMatchingTooManyDefinitionsLinksToNoneAndIsCounted]] (calls)
- [[internal/scan/resolve_test.go/TestClassLinesEmitInheritsAndImplementsEdges]] (calls)
- [[internal/scan/resolve_test.go/TestADeclaredReceiverTypeLinksOnlyWithinItsClassHierarchy]] (calls)
- [[internal/scan/resolve_test.go/TestADeclaredReceiverTypeImportedFromAFileLinksOnlyIntoIt]] (calls)
- [[internal/scan/resolve_test.go/TestASuperCallLinksToTheSupertypesMethod]] (calls)
- [[internal/scan/resolve_test.go/TestACallNarrowsToItsPackageAcrossSourceRoots]] (calls)
- [[internal/scan/resolve_test.go/TestResolutionEdgeCases]] (calls)

## Callers

none

## Callees

- [[internal/scan/scan.go/ScanFile]] (calls)
- [[internal/scan/scan.go/AssembleStats]] (calls)
