---
id: "cmd/rtdd/main_test.go::newTestRepo"
source: "cmd/rtdd/main_test.go"
start: 27
end: 44
kind: "func"
is-test: false
tags: [rtdd/code]
---
# newTestRepo

#rtdd/code

## Tests

- [[cmd/rtdd/agent_matrix_test.go/TestClaudeCodeProjectSkillIsTheProjectRender]] (calls)
- [[cmd/rtdd/doctor_test.go/TestDoctorWithAPositionalArgumentIsAUsageError]] (calls)
- [[cmd/rtdd/doctor_test.go/TestDoctorWithAnUnknownFlagIsAUsageError]] (calls)
- [[cmd/rtdd/doctor_test.go/TestUsageDocumentsDoctor]] (calls)
- [[cmd/rtdd/event_test.go/TestEventLogUnsetCreatesNoFile]] (calls)
- [[cmd/rtdd/event_test.go/TestEventLogLineStaysUnderTheBound]] (calls)
- [[cmd/rtdd/event_test.go/TestEventLogRecordsEachCommandsFields]] (calls)
- [[cmd/rtdd/init_test.go/TestInitInstallsIntoTheWorkingDirectory]] (calls)
- [[cmd/rtdd/init_test.go/TestInitProjectSkillWritesTheWholeFileFrontEndsOnly]] (calls)
- [[cmd/rtdd/init_test.go/TestInitAppendToMergesTheNamedSharedFile]] (calls)
- [[cmd/rtdd/init_test.go/TestInitReportsAnOlderInitsFootprintAndLeavesItAlone]] (calls)
- [[cmd/rtdd/init_test.go/TestInitDryRunPrintsThePlanAndWritesNothing]] (calls)
- [[cmd/rtdd/init_test.go/TestInitConflictsOnASkillFileRtddDidNotWrite]] (calls)
- [[cmd/rtdd/init_test.go/TestInitRejectsExtraArguments]] (calls)
- [[cmd/rtdd/init_test.go/TestUsageDocumentsInit]] (calls)
- [[cmd/rtdd/init_test.go/TestInitInstallsAtTheRepoRootFromASubdirectory]] (calls)
- [[cmd/rtdd/main_test.go/TestRunWithNoArgsIsAUsageError]] (calls)
- [[cmd/rtdd/main_test.go/TestRunWithAnUnknownCommandIsAUsageError]] (calls)
- [[cmd/rtdd/main_test.go/TestStatusWithAnUnknownFlagIsAUsageError]] (calls)
- [[cmd/rtdd/main_test.go/TestNothingExitsOne]] (calls)
- [[cmd/rtdd/main_test.go/TestWhichRejectsAnUnknownFlag]] (calls)
- [[cmd/rtdd/obsidian_test.go/TestObsidianNotePathsAreSafeAndUnique/TestObsidianInHelp]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallWritesTheClaudeSkill]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallSucceedsAndCreatesNothingWhenNoAgentIsInstalled]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallCreatesOnlyRtddOwnedDirectories]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallReportsAnOlderInstallsBlock]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillInstallDryRunWritesNothing]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillUninstallRemovesWhatInstallWrote]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillPromptIsSelfContainedAndNeedsNoInstalledAgent]] (calls)
- [[cmd/rtdd/skill_test.go/TestSkillRejectsAnUnknownSubcommand]] (calls)
- [[cmd/rtdd/skill_test.go/TestUninstallGlobalDryRunPrintsThePlanAndChangesNothing]] (calls)
- [[cmd/rtdd/skill_test.go/TestUninstallGlobalRemovesTheMachineWideSkill]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesRefusesTheOtherFlags]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestUpdateFromV030InstallsTheSkillForAnAgentTheNewReleaseSupports]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestUpdateLeavesTheRefreshToTheNewBinary]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestUpdateNotesARefreshTheNewBinaryCouldNotDo]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestAlreadyCurrentUpdateRestoresADeletedAgentSkill]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestUpdateCheckInstallsNoMissingSkill]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestInitInstallsAMissingAgentSkillAndRefreshesNothing]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestInitDryRunInstallsNoMissingSkill]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestInitRestoresTheCanonicalCopyADanglingLinkNeeds]] (calls)
- [[cmd/rtdd/update_test.go/TestUpdateCheckExitsZeroWhenANewerReleaseExists]] (calls)
- [[cmd/rtdd/update_test.go/TestUpdateFromASourceBuildIsAConfigurationError]] (calls)
- [[cmd/rtdd/update_test.go/TestUpdateReportsLegacyFiles]] (calls)
- [[cmd/rtdd/version_test.go/TestVersionPrintsTheBuildIdentity]] (calls)

## Callers

- [[cmd/rtdd/agent_matrix_test.go/rowProjectOptIn]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowUninstallAndLegacy]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowDoctorReports]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowSameInstructions]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowEventLog]] (calls)

## Callees

- [[.gitignore/_file]] (references)
- [[.harness/.gitignore/_file]] (references)
- [[bench/.gitignore/_file]] (references)
- [[internal/gitctx/gittest/gittest.go/Init]] (calls)
- [[internal/gitctx/gittest/gittest.go/Write]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
