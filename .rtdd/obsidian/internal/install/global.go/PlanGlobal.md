---
id: "internal/install/global.go::PlanGlobal"
source: "internal/install/global.go"
start: 117
end: 172
kind: "func"
is-test: false
tags: [rtdd/code]
---
# PlanGlobal

#rtdd/code

## Tests

- [[internal/install/agents_test.go/TestAgentStatusReadsTheHome]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalSkipsAgentsThatAreNotInstalled]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesTheClaudeSkillWhenClaudeIsInstalled]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesOnlyRtddOwnedDirectories]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesCodexToTheSharedAgentsDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesAGeminiExtension]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalConflictsOnAnEditedSkillAndForceOverwrites]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRestoresTheHostsFiles]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalReplacesItsOwnPreviousSkillWithoutForce]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLinksHermesToTheCanonicalSkill]] (calls)
- [[internal/install/link_test.go/TestRefreshingTheCanonicalCopyRefreshesEveryLinkedAgent]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAHandWrittenFileAtALinkedPathAlone]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAForeignLinkAlone]] (calls)

## Callers

- [[cmd/rtdd/skill.go/cmdSkillInstall]] (calls)
- [[cmd/rtdd/skill.go/refreshGlobalSkill]] (calls)
- [[internal/install/link_test.go/installGlobal]] (calls)

## Callees

- [[internal/install/global.go/GlobalSurfaces]] (calls)
- [[internal/install/global.go/planGlobalFile]] (calls)
- [[internal/install/global.go/planGlobalLink]] (calls)
- [[internal/install/global.go/globalSkip]] (calls)
- [[internal/install/global.go/planRemoveDir]] (calls)
- [[internal/install/global.go/generatedFile]] (calls)
