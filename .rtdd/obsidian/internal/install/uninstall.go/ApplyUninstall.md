---
id: "internal/install/uninstall.go::ApplyUninstall"
source: "internal/install/uninstall.go"
start: 232
end: 262
kind: "func"
is-test: false
tags: [rtdd/code]
---
# ApplyUninstall

#rtdd/code

## Tests

- [[internal/install/global_test.go/TestPlanGlobalUninstallRestoresTheHostsFiles]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallKeepsADirectoryHoldingAForeignFile]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallRemovesARetiredDirectory]] (calls)
- [[internal/install/global_test.go/TestPlanGlobalUninstallNeverPlansTheHomeDirectory]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesStripTheOldBlockAndLeaveTheUsersTextByteIdentical]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesDeleteTheOldWholeFileFrontEnds]] (calls)
- [[internal/install/legacy_test.go/TestLegacyFilesMoveTheGraphCacheLineIntoRtdd]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesStripTheBlockFromTheSharedHomeFiles]] (calls)
- [[internal/install/legacy_test.go/TestGlobalLegacyFilesReportRtddsFilesInARetiredDirectory]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAForeignLinkAlone]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalUninstallRemovesLinksAndTheCanonicalCopy]] (calls)
- [[internal/install/uninstall_test.go/TestApplyUninstallLeavesTheHostsOwnContent]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsAFrontEndRtddDidNotWrite]] (calls)
- [[internal/install/uninstall_test.go/TestUninstallingTwiceIsNotAnError]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallRefusesAnAmbiguousAgentsFile]] (calls)

## Callers

- [[cmd/rtdd/skill.go/cmdSkillUninstall]] (calls)
- [[cmd/rtdd/uninstall.go/cmdUninstall]] (calls)
- [[cmd/rtdd/uninstall.go/uninstallLegacy]] (calls)

## Callees

none
