---
id: "internal/install/install.go::Plan"
source: "internal/install/install.go"
start: 70
end: 159
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Plan

#rtdd/code

## Tests

- [[internal/install/install_test.go/TestPlanOnACleanRepoWritesOnlyInsideRtdd]] (calls)
- [[internal/install/install_test.go/TestPlanNeverTouchesTheHostsSharedFiles]] (calls)
- [[internal/install/install_test.go/TestPlanProjectSkillWritesTheWholeFileFrontEndsMarkedOptIn]] (calls)
- [[internal/install/install_test.go/TestPlanProjectSkillFromHomeWritesNoUserLevelPath]] (calls)
- [[internal/install/install_test.go/TestPlanAppendToMergesTheBlockIntoTheNamedFileOnly]] (calls)
- [[internal/install/install_test.go/TestPlanAppendToRefusesAPathOutsideTheRepositoryOrInsideRtdd]] (calls)
- [[internal/install/install_test.go/TestPlanSkipsWholeFileTargetsWhenContentIsUnchanged]] (calls)
- [[internal/install/install_test.go/TestPlanRtddGitignoreAppendsTheGraphLineOnce]] (calls)
- [[internal/install/install_test.go/TestPlanLeavesGitattributesAlone]] (calls)
- [[internal/install/install_test.go/TestPlanConfigIsCreatedOnceThenNeverOverwritten]] (calls)
- [[internal/install/install_test.go/TestApplyIsANoOpOnAllSkipStepsAndWritesEverythingElse]] (calls)
- [[internal/install/migrate_test.go/TestPlanReplacesAV02OnlyConfigAndKeepsAV030One]] (calls)
- [[internal/install/migrate_test.go/TestPlanKeepsAConfigSettingWholeFileTests]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanWritesTheConfigAndIgnoresTheGraphCache]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanNeverPlansTheRootGitignore]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile]] (calls)

## Callers

- [[cmd/rtdd/init.go/cmdInit]] (calls)

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[internal/install/config.go/DefaultConfig]] (calls)
- [[internal/install/install.go/planRtddGitignore]] (calls)
- [[internal/install/install.go/markOptIn]] (calls)
- [[internal/install/install.go/repoRel]] (calls)
- [[internal/install/install.go/sortedKeys]] (calls)
- [[internal/install/legacy.go/PlanLegacyFiles]] (calls)
- [[internal/install/legacy.go/ReportLegacy]] (calls)
- [[internal/install/merge.go/MergeBlock]] (calls)
- [[internal/install/migrate.go/PlanMigration]] (calls)
- [[internal/install/migrate.go/isV02Config]] (calls)
- [[internal/install/uninstall.go/samePath]] (calls)
- [[internal/install/uninstall.go/userLevel]] (calls)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
