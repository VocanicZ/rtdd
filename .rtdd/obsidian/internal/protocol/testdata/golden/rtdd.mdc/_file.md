---
id: "internal/protocol/testdata/golden/rtdd.mdc"
source: "internal/protocol/testdata/golden/rtdd.mdc"
start: 1
end: 78
kind: "file"
is-test: false
tags: [rtdd/file]
---
# rtdd.mdc

#rtdd/file

## Tests

- [[cmd/rtdd-gen/main_test.go/TestVerifyCatchesStrippedFrontmatter]] (references)
- [[cmd/rtdd-gen/main_test.go/TestVerifyCatchesOverBudgetMDC]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestAgentMatrix]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestAgentMatrixColumnsAreTheAgentsTable]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestEveryVerifiedColumnSaysSo]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestEveryAgentInScopeIsAVerifiedMatrixColumn]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestClaudeCodeProjectSkillIsTheProjectRender]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallForAntigravityAloneWritesNoGeminiExtension]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestGeminiExtensionManifestNamesItsContextFile]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestCursorGetsAUserLevelSkill]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestOpenCodeSkillNameEqualsItsDirectory]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestZcodeHasNoProjectSurface]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallLeavesHermesConfigAlone]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallNeverWritesCopilotInstructions]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallRetiresTheOldCodexSkillDirectory]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallDoesNotReportTheDirectoryItRetiresAsLegacy]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestRefreshGlobalSkillSaysItRemovedTheOldCodexSkill]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestSkillInstallWithEveryAgentPresentTouchesNoUserFile]] (references)
- [[cmd/rtdd/agent_matrix_test.go/TestAgentMatrixLinksAreTheAgentsTable]] (references)
- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsLegacyFiles]] (references)
- [[cmd/rtdd/init_shapes_test.go/TestWhichAfterAFreshInitNamesNothingUnderCursor]] (references)
- [[cmd/rtdd/init_test.go/TestRenderInit]] (references)
- [[cmd/rtdd/init_test.go/TestInitProjectSkillWritesTheWholeFileFrontEndsOnly]] (references)
- [[cmd/rtdd/uninstall_test.go/TestUninstallRemovesWhatInitWrote]] (references)
- [[cmd/rtdd/uninstall_test.go/TestRenderUninstallListsEveryStep]] (references)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesRemovesExactlyTheOldFootprint]] (references)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesStopsOnAConflictWithNothingChanged]] (references)
- [[internal/install/agents_test.go/TestAgentsTableInvariants]] (references)
- [[internal/install/install_test.go/TestPlanProjectSkillWritesTheWholeFileFrontEndsMarkedOptIn]] (references)
- [[internal/install/install_test.go/TestApplyIsANoOpOnAllSkipStepsAndWritesEverythingElse]] (references)
- [[internal/install/install_test.go/TestFilesReturnsTheEmbeddedGeneratedFrontEnds]] (references)
- [[internal/install/legacy_test.go/TestLegacyFilesLeaveOptInsAndForeignFilesAlone]] (references)
- [[internal/install/legacy_test.go/TestLegacyFilesDeleteTheOldWholeFileFrontEnds]] (references)
- [[internal/install/plan_v030_test.go/TestPlanReplacesAnEarlierRenderAndConflictsOnAForeignFile]] (references)
- [[internal/installtest/release_artifacts_test.go/TestReleaseArchivesShipTheLicenseAndReadme]] (references)
- [[internal/installtest/release_snapshot_test.go/TestTheArchiveGateRunsWithNoGoreleaserBinaryOnPATH]] (references)
- [[internal/installtest/release_snapshot_test.go/TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath]] (references)
- [[internal/installtest/release_snapshot_test.go/TestArchiveContentCheckRejectsAnArchiveMissingAShippedPath]] (references)
- [[internal/installtest/release_snapshot_test.go/TestArchiveContentCheckRejectsAnArchiveMissingAShippedPath]] (references)
- [[internal/installtest/release_snapshot_test.go/TestArchiveReaderRejectsTheWrongFormat]] (references)
- [[internal/protocol/render_test.go/TestRenderAllKeysByOutPath]] (references)
- [[internal/protocol/render_test.go/TestRenderAllTargetsGetDifferentSections]] (references)
- [[internal/protocol/render_test.go/TestRenderMDCFrontmatterHasGlobsAndAlwaysApply]] (references)

## Callers

- [[.goreleaser.yaml/_file]] (references)
- [[cmd/rtdd/uninstall_test.go/legacyRepo]] (references)
- [[docs/plans/00-interfaces.md/_file]] (references)
- [[docs/plans/03-m2-signal.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/06-m6a-adapter-contract-v2.md/_file]] (references)
- [[docs/plans/07-m7-release-readiness.md/_file]] (references)
- [[docs/plans/11-skill-install.md/_file]] (references)
- [[docs/plans/2026-10-24-every-agent.md/_file]] (references)
- [[docs/reference.md/_file]] (references)
- [[docs/specs/2026-08-26-rtdd-design.md/_file]] (references)
- [[docs/specs/2026-09-05-multi-language.md/_file]] (references)
- [[internal/install/agents.go/Agents]] (references)
- [[internal/install/install_test.go/fakeFiles]] (references)
- [[internal/install/legacy.go/PlanLegacyFiles]] (references)
- [[internal/protocol/targets.go/_file]] (references)

## Callees

- [[docs/results/worked-example-v3/graph.json/_file]] (references)
- [[internal/graphify/testdata/graphify-out/graph.json/_file]] (references)
- [[protocol/PROTOCOL.md/_file]] (references)
