---
id: "internal/installtest/install_test.go::runInstaller"
source: "internal/installtest/install_test.go"
start: 115
end: 146
kind: "func"
is-test: false
tags: [rtdd/code]
---
# runInstaller

#rtdd/code

## Tests

- [[internal/installtest/install_skill_test.go/TestInstallWritesTheAgentSkill]] (calls)
- [[internal/installtest/install_skill_test.go/TestInstallHonoursRTDDNoSkill]] (calls)
- [[internal/installtest/install_skill_test.go/TestInstallSucceedsWhenTheSkillInstallFails]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesResolvesTheLatestRelease]] (calls)
- [[internal/installtest/install_snapshot_test.go/TestInstallFromRealSnapshotArchivesAbortsOnACorruptedChecksum]] (calls)
- [[internal/installtest/install_test.go/TestInstallOnACleanPath]] (calls)
- [[internal/installtest/install_test.go/TestInstallAbortsOnACorruptedChecksum]] (calls)

## Callers

- [[internal/installtest/install_test.go/runUnpinnedInstall]] (calls)

## Callees

- [[installer/index.js/_file]] (references)
