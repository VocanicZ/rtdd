---
id: "internal/install/uninstall.go::userLevel"
source: "internal/install/uninstall.go"
start: 208
end: 215
kind: "func"
is-test: false
tags: [rtdd/code]
---
# userLevel

#rtdd/code

## Tests

- [[internal/install/install_test.go/TestPlanProjectSkillFromHomeWritesNoUserLevelPath]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallFromHomeReportsAMissingUserLevelFileAsNotPresent]] (calls)

## Callers

- [[internal/install/install.go/Plan]] (calls)
- [[internal/install/legacy.go/PlanLegacyFiles]] (calls)
- [[internal/install/uninstall.go/PlanUninstall]] (calls)

## Callees

- [[internal/install/global.go/GlobalSurfaces]] (calls)
