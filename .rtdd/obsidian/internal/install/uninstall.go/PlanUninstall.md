---
id: "internal/install/uninstall.go::PlanUninstall"
source: "internal/install/uninstall.go"
start: 80
end: 157
kind: "func"
is-test: false
tags: [rtdd/code]
---
# PlanUninstall

#rtdd/code

## Tests

- [[internal/install/uninstall_test.go/TestPlanUninstallRemovesExactlyWhatPlanWrites]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsTheRecordedMapUnlessAsked]] (calls)
- [[internal/install/uninstall_test.go/TestApplyUninstallLeavesTheHostsOwnContent]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsAFrontEndRtddDidNotWrite]] (calls)
- [[internal/install/uninstall_test.go/TestUninstallingTwiceIsNotAnError]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallRefusesAnAmbiguousAgentsFile]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallFromHomeReportsAMissingUserLevelFileAsNotPresent]] (calls)

## Callers

- [[cmd/rtdd/uninstall.go/cmdUninstall]] (calls)

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[internal/install/install.go/sortedKeys]] (calls)
- [[internal/install/uninstall.go/RemoveBlock]] (calls)
- [[internal/install/uninstall.go/uninstallWholeFile]] (calls)
- [[internal/install/uninstall.go/samePath]] (calls)
- [[internal/install/uninstall.go/userLevel]] (calls)
- [[internal/install/uninstall.go/removeLine]] (calls)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
