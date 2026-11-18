---
id: "internal/install/global_test.go::globalFiles"
source: "internal/install/global_test.go"
start: 14
end: 22
kind: "func"
is-test: false
tags: [rtdd/code]
---
# globalFiles

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

- [[internal/install/link_test.go/installGlobal]] (calls)

## Callees

none
