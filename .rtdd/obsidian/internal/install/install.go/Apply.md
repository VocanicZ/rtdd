---
id: "internal/install/install.go::Apply"
source: "internal/install/install.go"
start: 218
end: 247
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Apply

#rtdd/code

## Tests

- [[internal/install/agents_test.go/TestAgentStatusReadsTheHome]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalSkipsAgentsThatAreNotInstalled]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesTheClaudeSkillWhenClaudeIsInstalled]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesOnlyRtddOwnedDirectories]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesCodexToTheSharedAgentsDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesAGeminiExtension]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRestoresTheHostsFiles]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalReplacesItsOwnPreviousSkillWithoutForce]] (calls)
- [[internal/install/install_test.go/TestPlanOnACleanRepoWritesOnlyInsideRtdd]] (calls)
- [[internal/install/install_test.go/TestPlanNeverTouchesTheHostsSharedFiles]] (calls)
- [[internal/install/install_test.go/TestPlanRtddGitignoreAppendsTheGraphLineOnce]] (calls)
- [[internal/install/install_test.go/TestPlanLeavesGitattributesAlone]] (calls)
- [[internal/install/install_test.go/TestPlanConfigIsCreatedOnceThenNeverOverwritten]] (calls)
- [[internal/install/install_test.go/TestApplyIsANoOpOnAllSkipStepsAndWritesEverythingElse]] (calls)
- [[internal/install/install_test.go/TestApplyRefusesToWriteOnConflict]] (calls)
- [[internal/install/legacy_test.go/TestReportLegacyIsAReadOnlyLinePerPiece]] (calls)
- [[internal/install/link_test.go/TestRefreshingTheCanonicalCopyRefreshesEveryLinkedAgent]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesEveryV02Artefact]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesTheV01ReportAndTheAdapterOverride]] (calls)

## Callers

- [[cmd/rtdd/init.go/cmdInit]] (calls)
- [[cmd/rtdd/skill.go/cmdSkillInstall]] (calls)
- [[cmd/rtdd/skill.go/refreshGlobalSkill]] (calls)
- [[internal/install/link_test.go/installGlobal]] (calls)

## Callees

- [[internal/install/install.go/applyLink]] (calls)
