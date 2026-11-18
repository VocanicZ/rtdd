---
id: "internal/install/uninstall_test.go::installedRepo"
source: "internal/install/uninstall_test.go"
start: 86
end: 112
kind: "func"
is-test: false
tags: [rtdd/code]
---
# installedRepo

#rtdd/code

## Tests

- [[internal/install/uninstall_test.go/TestPlanUninstallRemovesExactlyWhatPlanWrites]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsTheRecordedMapUnlessAsked]] (calls)
- [[internal/install/uninstall_test.go/TestApplyUninstallLeavesTheHostsOwnContent]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallKeepsAFrontEndRtddDidNotWrite]] (calls)
- [[internal/install/uninstall_test.go/TestUninstallingTwiceIsNotAnError]] (calls)
- [[internal/install/uninstall_test.go/TestPlanUninstallRefusesAnAmbiguousAgentsFile]] (calls)

## Callers

none

## Callees

- [[docs/results/worked-example/map.jsonl/_file]] (references)
- [[internal/gitctx/helper_test.go/write]] (calls)
- [[internal/install/install.go/sortedKeys]] (calls)
- [[internal/install/merge.go/MergeBlock]] (calls)
- [[internal/install/uninstall_test.go/testBlock]] (calls)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
- [[internal/scan/filter_test.go/write]] (calls)
