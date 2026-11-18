---
id: "internal/scan/rules_test.go::linkAll"
source: "internal/scan/rules_test.go"
start: 52
end: 64
kind: "func"
is-test: false
tags: [rtdd/code]
---
# linkAll

#rtdd/code

## Tests

- [[internal/scan/rules_test.go/TestACallLinksToSameNamedDefinitionsOfItsOwnFileTypeFirst]] (calls)
- [[internal/scan/rules_test.go/TestACallFallsBackToOtherFileTypesWhenItsOwnHasNoDefinition]] (calls)
- [[internal/scan/rules_test.go/TestTheFallbackReachesOnlyExtensionsThatShareADirectoryWithTheCaller]] (calls)

## Callers

none

## Callees

- [[internal/scan/link.go/Link]] (calls)
- [[internal/scan/scan.go/ScanFile]] (calls)
