---
id: "internal/install/uninstall.go::RemoveBlock"
source: "internal/install/uninstall.go"
start: 23
end: 58
kind: "func"
is-test: false
tags: [rtdd/code]
---
# RemoveBlock

#rtdd/code

## Tests

- [[internal/install/uninstall_test.go/TestMergeThenRemoveReturnsTheFileUnchanged]] (calls)
- [[internal/install/uninstall_test.go/TestRemoveBlockLeavesAFileWithoutMarkersAlone]] (calls)
- [[internal/install/uninstall_test.go/TestRemoveBlockRefusesAnAmbiguousFile]] (calls)

## Callers

- [[internal/install/legacy.go/legacyBlock]] (calls)
- [[internal/install/uninstall.go/PlanUninstall]] (calls)

## Callees

none
