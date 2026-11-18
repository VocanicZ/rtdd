---
id: "internal/scan/resolve_test.go::edgesOf"
source: "internal/scan/resolve_test.go"
start: 21
end: 29
kind: "func"
is-test: false
tags: [rtdd/code]
---
# edgesOf

#rtdd/code

## Tests

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

- [[internal/scan/fallback_test.go/callsFrom]] (calls)

## Callees

none
