---
id: "internal/install/install_test.go::fakeFiles"
source: "internal/install/install_test.go"
start: 12
end: 18
kind: "func"
is-test: false
tags: [rtdd/code]
---
# fakeFiles

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

none

## Callees

- [[internal/install/merge_test.go/block]] (calls)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
- [[internal/protocol/testdata/golden/SKILL.md/_file]] (references)
- [[internal/protocol/testdata/golden/rtdd.mdc/_file]] (references)
