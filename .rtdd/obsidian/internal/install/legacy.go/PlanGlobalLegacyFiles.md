---
id: "internal/install/legacy.go::PlanGlobalLegacyFiles"
source: "internal/install/legacy.go"
start: 111
end: 138
kind: "func"
is-test: false
tags: [rtdd/code]
---
# PlanGlobalLegacyFiles

#rtdd/code

## Tests

- [[internal/install/legacy_test.go/TestGlobalLegacyFilesStripTheBlockFromTheSharedHomeFiles]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesReportRtddsFilesInARetiredDirectory]] (calls)

## Callers

- [[cmd/rtdd/legacy.go/globalLegacy]] (calls)
- [[cmd/rtdd/uninstall.go/uninstallLegacy]] (calls)

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[internal/install/agents.go/Agents]] (calls)
- [[internal/install/global.go/planRemoveDir]] (calls)
- [[internal/install/global.go/generatedFile]] (calls)
- [[internal/install/legacy.go/legacyBlock]] (calls)
