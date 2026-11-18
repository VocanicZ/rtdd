---
id: "cmd/rtdd/main_test.go::writeFile"
source: "cmd/rtdd/main_test.go"
start: 14
end: 17
kind: "func"
is-test: false
tags: [rtdd/code]
---
# writeFile

#rtdd/code

## Tests

- [[cmd/rtdd/agent_matrix_test.go/TestGeminiExtensionManifestNamesItsContextFile]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallLeavesHermesConfigAlone]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallRetiresTheOldCodexSkillDirectory]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallDoesNotReportTheDirectoryItRetiresAsLegacy]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/TestRefreshGlobalSkillSaysItRemovedTheOldCodexSkill]] (calls)
- [[cmd/rtdd/init_migrate_test.go/TestInitOnAFreshRepositoryPrintsNoRemoval]] (calls)
- [[cmd/rtdd/init_test.go/TestInitInstallsIntoTheWorkingDirectory]] (calls)
- [[cmd/rtdd/init_test.go/TestInitProjectSkillWritesTheWholeFileFrontEndsOnly]] (calls)
- [[cmd/rtdd/init_test.go/TestInitAppendToMergesTheNamedSharedFile]] (calls)
- [[cmd/rtdd/init_test.go/TestInitReportsAnOlderInitsFootprintAndLeavesItAlone]] (calls)
- [[cmd/rtdd/init_test.go/TestInitConflictsOnASkillFileRtddDidNotWrite]] (calls)
- [[cmd/rtdd/main_test.go/TestNothingExitsOne]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallCreatesOnlyRtddOwnedDirectories]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallReportsAnOlderInstallsBlock]] (calls)
- [[cmd/rtdd/skill_test.go/TestUnplannedLegacyDropsWhatThePlanDeletes]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesStopsOnAConflictWithNothingChanged]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestSkillInstallRefreshLeavesAHandWrittenSkillAloneAndExitsZero]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestInitInstallsAMissingAgentSkillAndRefreshesNothing]] (calls)
- [[cmd/rtdd/update_test.go/TestUpdateReportsLegacyFiles]] (calls)

## Callers

- [[cmd/rtdd/agent_matrix_test.go/seedAgent]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowUpdateRefreshes]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowUninstallAndLegacy]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowEventLog]] (calls)
- [[cmd/rtdd/init_migrate_test.go/v02Repository]] (calls)
- [[cmd/rtdd/init_test.go/markDetectable]] (calls)
- [[cmd/rtdd/uninstall_test.go/legacyRepo]] (calls)
- [[cmd/rtdd/update_refresh_test.go/v030Home]] (calls)

## Callees

- [[internal/gitctx/gittest/gittest.go/Write]] (calls)
