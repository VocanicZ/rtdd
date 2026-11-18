---
id: ".gitignore"
source: ".gitignore"
start: 1
end: 18
kind: "file"
is-test: false
tags: [rtdd/file]
---
# .gitignore

#rtdd/file

## Tests

- [[bench/tests/test_cache.py/test_the_cache_root_defaults_to_bench_cache_and_is_gitignored]] (references)
- [[cmd/rtdd/init_migrate_test.go/TestInitMigratesAV02Repository]] (references)
- [[cmd/rtdd/init_shapes_test.go/TestInitSucceedsOnEveryRepositoryShape]] (references)
- [[cmd/rtdd/init_shapes_test.go/TestInitTwiceIgnoresTheGraphCacheExactlyOnce]] (references)
- [[cmd/rtdd/init_test.go/TestInitInstallsIntoTheWorkingDirectory]] (references)
- [[cmd/rtdd/init_test.go/TestInitReportsAnOlderInitsFootprintAndLeavesItAlone]] (references)
- [[cmd/rtdd/init_test.go/TestInitDryRunPrintsThePlanAndWritesNothing]] (references)
- [[cmd/rtdd/init_test.go/TestInitInstallsAtTheRepoRootFromASubdirectory]] (references)
- [[cmd/rtdd/init_test.go/TestInitFallsBackToTheWorkingDirectoryOutsideAGitRepository]] (references)
- [[cmd/rtdd/init_test.go/TestInitIgnoresAStrayGitAncestorAndInstallsIntoTheWorkingDirectory]] (references)
- [[cmd/rtdd/obsidian_test.go/TestObsidianNoOpenWritesOneNotePerNode]] (references)
- [[cmd/rtdd/readme_example_test.go/TestREADMEWhichExampleIsRealOutput]] (references)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesRemovesExactlyTheOldFootprint]] (references)
- [[internal/contract/contract_test.go/TestGitIgnoreIgnoresBuildOutput]] (references)
- [[internal/gitctx/listfiles_test.go/TestListFilesIsTrackedPlusUntrackedMinusIgnoredAndDeleted]] (references)
- [[internal/install/install_test.go/TestPlanOnACleanRepoWritesOnlyInsideRtdd]] (references)
- [[internal/install/install_test.go/TestPlanNeverTouchesTheHostsSharedFiles]] (references)
- [[internal/install/install_test.go/TestPlanRtddGitignoreAppendsTheGraphLineOnce]] (references)
- [[internal/install/install_test.go/TestApplyIsANoOpOnAllSkipStepsAndWritesEverythingElse]] (references)
- [[internal/install/legacy_test.go/TestLegacyFilesLeaveOptInsAndForeignFilesAlone]] (references)
- [[internal/install/legacy_test.go/TestLegacyFilesMoveTheGraphCacheLineIntoRtdd]] (references)
- [[internal/install/legacy_test.go/TestReportLegacyIsAReadOnlyLinePerPiece]] (references)
- [[internal/install/plan_v030_test.go/TestPlanWritesTheConfigAndIgnoresTheGraphCache]] (references)
- [[internal/install/plan_v030_test.go/TestPlanNeverPlansTheRootGitignore]] (references)
- [[internal/installtest/readme_install_test.go/TestInstallEditLeavesTheProductClaimsAlone]] (references)
- [[internal/installtest/release_snapshot_test.go/TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[README.md/_file]] (references)
- [[cmd/rtdd/detect_test.go/newDetectableRepo]] (references)
- [[cmd/rtdd/init_next_test.go/newPolyglotRepo]] (references)
- [[cmd/rtdd/main_test.go/newTestRepo]] (references)
- [[cmd/rtdd/obsidian.go/ignoreVault]] (references)
- [[cmd/rtdd/uninstall_test.go/legacyRepo]] (references)
- [[docs/outcomes/README.positive.md/_file]] (references)
- [[docs/plans/01-m1a-core.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/07-m7-release-readiness.md/_file]] (references)
- [[docs/plans/08-one-pipeline.md/_file]] (references)
- [[docs/plans/09-node-graph-core.md/_file]] (references)
- [[docs/plans/10-rounds-cutover.md/_file]] (references)
- [[docs/plans/11-skill-install.md/_file]] (references)
- [[docs/reference.md/_file]] (references)
- [[docs/results/worked-example-v3/repo.json/_file]] (references)
- [[internal/install/install.go/planRtddGitignore]] (references)
- [[internal/install/legacy.go/PlanLegacyFiles]] (references)
- [[internal/install/protocol.md/_file]] (references)
- [[internal/installtest/install_snapshot_test.go/requireSnapshotRelease]] (references)
- [[internal/installtest/release_snapshot_test.go/buildDistIsGitIgnored]] (references)
- [[internal/protocol/testdata/golden/GLOBAL-SKILL.md/_file]] (references)
- [[protocol/PROTOCOL.md/_file]] (references)

## Callees

none
