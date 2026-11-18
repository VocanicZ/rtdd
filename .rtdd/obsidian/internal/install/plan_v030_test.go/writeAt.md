---
id: "internal/install/plan_v030_test.go::writeAt"
source: "internal/install/plan_v030_test.go"
start: 11
end: 20
kind: "func"
is-test: false
tags: [rtdd/code]
---
# writeAt

#rtdd/code

## Tests

- [[internal/install/global_test.go/TestPlanGlobalWritesOnlyRtddOwnedDirectories]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalWritesAGeminiExtension]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRestoresTheHostsFiles]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallKeepsADirectoryHoldingAForeignFile]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRemovesARetiredDirectory]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesStripTheOldBlockAndLeaveTheUsersTextByteIdentical]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesLeaveOptInsAndForeignFilesAlone]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesDeleteTheOldWholeFileFrontEnds]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesMoveTheGraphCacheLineIntoRtdd]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesStripTheBlockFromTheSharedHomeFiles]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesReportRtddsFilesInARetiredDirectory]] (calls)
- [[internal/install/legacy_test.go/TestReportLegacyIsAReadOnlyLinePerPiece]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesTheV01ReportAndTheAdapterOverride]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationOnAFreshRepositoryPlansNothing]] (calls)
- [[internal/install/migrate_test.go/TestPlanReplacesAV02OnlyConfigAndKeepsAV030One]] (calls)
- [[internal/install/migrate_test.go/TestPlanKeepsAConfigSettingWholeFileTests]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanNeverPlansTheRootGitignore]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile]] (calls)

## Callers

- [[internal/install/migrate_test.go/v02Layout]] (calls)

## Callees

none
