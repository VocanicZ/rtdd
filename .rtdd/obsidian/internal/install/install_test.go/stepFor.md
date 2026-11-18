---
id: "internal/install/install_test.go::stepFor"
source: "internal/install/install_test.go"
start: 20
end: 29
kind: "func"
is-test: false
tags: [rtdd/code]
---
# stepFor

#rtdd/code

## Tests

- [[internal/install/global_test.go/TestPlanGlobalWritesTheClaudeSkillWhenClaudeIsInstalled]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesCodexToTheSharedAgentsDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalConflictsOnAnEditedSkillAndForceOverwrites]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRemovesARetiredDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalReplacesItsOwnPreviousSkillWithoutForce]] (calls)
- [[internal/install/install_test.go/TestPlanOnACleanRepoWritesOnlyInsideRtdd]] (calls)
- [[internal/install/install_test.go/TestPlanProjectSkillWritesTheWholeFileFrontEndsMarkedOptIn]] (calls)
- [[internal/install/install_test.go/TestPlanProjectSkillFromHomeWritesNoUserLevelPath]] (calls)
- [[internal/install/install_test.go/TestPlanAppendToMergesTheBlockIntoTheNamedFileOnly]] (calls)
- [[internal/install/install_test.go/TestPlanAppendToRefusesAPathOutsideTheRepositoryOrInsideRtdd]] (calls)
- [[internal/install/install_test.go/TestPlanSkipsWholeFileTargetsWhenContentIsUnchanged]] (calls)
- [[internal/install/install_test.go/TestPlanRtddGitignoreAppendsTheGraphLineOnce]] (calls)
- [[internal/install/install_test.go/TestPlanConfigIsCreatedOnceThenNeverOverwritten]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesStripTheOldBlockAndLeaveTheUsersTextByteIdentical]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesDeleteTheOldWholeFileFrontEnds]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesStripTheBlockFromTheSharedHomeFiles]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesReportRtddsFilesInARetiredDirectory]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesEveryV02Artefact]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesTheV01ReportAndTheAdapterOverride]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationDeletesAGitattributesHoldingOnlyTheV02Line]] (calls)
- [[internal/install/migrate_test.go/TestPlanReplacesAV02OnlyConfigAndKeepsAV030One]] (calls)
- [[internal/install/migrate_test.go/TestPlanKeepsAConfigSettingWholeFileTests]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanWritesTheConfigAndIgnoresTheGraphCache]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallRemovesExactlyWhatPlanWrites]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsTheRecordedMapUnlessAsked]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsAFrontEndRtddDidNotWrite]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallRefusesAnAmbiguousAgentsFile]] (calls)

## Callers

none

## Callees

none
