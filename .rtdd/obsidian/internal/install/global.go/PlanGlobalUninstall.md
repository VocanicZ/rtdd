---
id: "internal/install/global.go::PlanGlobalUninstall"
source: "internal/install/global.go"
start: 285
end: 318
kind: "func"
is-test: false
tags: [rtdd/code]
---
# PlanGlobalUninstall

#rtdd/code

## Tests

- [[internal/install/global_test.go/TestPlanGlobalUninstallRestoresTheHostsFiles]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallKeepsADirectoryHoldingAForeignFile]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRemovesARetiredDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallNeverPlansTheHomeDirectory]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAForeignLinkAlone]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalUninstallRemovesLinksAndTheCanonicalCopy]] (calls)

## Callers

- [[cmd/rtdd/skill.go/cmdSkillUninstall]] (calls)
- [[cmd/rtdd/uninstall.go/cmdUninstall]] (calls)

## Callees

- [[internal/install/global.go/GlobalSurfaces]] (calls)
- [[internal/install/global.go/planRemoveCanonical]] (calls)
- [[internal/install/global.go/planRemoveDir]] (calls)
- [[internal/install/global.go/ownedFile]] (calls)
- [[internal/install/global.go/generatedFile]] (calls)
