---
id: "internal/install/global_test.go::homeWith"
source: "internal/install/global_test.go"
start: 25
end: 34
kind: "func"
is-test: false
tags: [rtdd/code]
---
# homeWith

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
- [[internal/install/global_test.go/TestPlanGlobalUninstallKeepsADirectoryHoldingAForeignFile]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRemovesARetiredDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallNeverPlansTheHomeDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalReplacesItsOwnPreviousSkillWithoutForce]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLinksHermesToTheCanonicalSkill]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalWritesNoCanonicalCopyWhenNothingLinks]] (calls)
- [[internal/install/link_test.go/TestRefreshingTheCanonicalCopyRefreshesEveryLinkedAgent]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalReplacesAnEarlierCopyWithALink]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAHandWrittenFileAtALinkedPathAlone]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAForeignLinkAlone]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalCopiesWhenTheCanonicalPathIsNotRtdds]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalUninstallRemovesLinksAndTheCanonicalCopy]] (calls)

## Callers

none

## Callees

none
