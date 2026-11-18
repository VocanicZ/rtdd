---
id: "internal/install/legacy.go::PlanLegacyFiles"
source: "internal/install/legacy.go"
start: 39
end: 106
kind: "func"
is-test: false
tags: [rtdd/code]
---
# PlanLegacyFiles

#rtdd/code

## Tests

- [[internal/install/legacy_test.go/TestLegacyFilesStripTheOldBlockAndLeaveTheUsersTextByteIdentical]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesLeaveOptInsAndForeignFilesAlone]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesDeleteTheOldWholeFileFrontEnds]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesMoveTheGraphCacheLineIntoRtdd]] (calls)
- [[internal/install/legacy_test.go/TestReportLegacyIsAReadOnlyLinePerPiece]] (calls)

## Callers

- [[cmd/rtdd/legacy.go/repoLegacy]] (calls)
- [[cmd/rtdd/uninstall.go/uninstallLegacy]] (calls)
- [[internal/install/install.go/Plan]] (calls)

## Callees

- [[.gitignore/_file]] (references)
- [[.harness/.gitignore/_file]] (references)
- [[bench/.gitignore/_file]] (references)
- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[internal/install/install.go/planRtddGitignore]] (calls)
- [[internal/install/install.go/repoRel]] (calls)
- [[internal/install/install.go/sortedKeys]] (calls)
- [[internal/install/legacy.go/legacyWholeFile]] (calls)
- [[internal/install/legacy.go/legacyBlock]] (calls)
- [[internal/install/uninstall.go/samePath]] (calls)
- [[internal/install/uninstall.go/userLevel]] (calls)
- [[internal/install/uninstall.go/removeLine]] (calls)
- [[internal/protocol/testdata/golden/SKILL.md/_file]] (references)
- [[internal/protocol/testdata/golden/rtdd.mdc/_file]] (references)
