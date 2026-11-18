---
id: "cmd/rtdd/uninstall_test.go::initialisedRepo"
source: "cmd/rtdd/uninstall_test.go"
start: 18
end: 28
kind: "func"
is-test: false
tags: [rtdd/code]
---
# initialisedRepo

#rtdd/code

## Tests

- [[cmd/rtdd/uninstall_test.go/TestUninstallRemovesWhatInitWrote]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallKeepsTheMapAndTheBinaryByDefault]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallStateRemovesTheRecordedMap]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallDryRunChangesNothing]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallIsIdempotent]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallStopsOnAFileItCannotRead]] (calls)
- [[cmd/rtdd/uninstall_test.go/TestUninstallLegacyFilesLeavesTheOptIns]] (calls)

## Callers

none

## Callees

- [[bench/pyproject.toml/_file]] (references)
- [[bench/swebench/pyproject.toml/_file]] (references)
- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[internal/gitctx/gittest/gittest.go/Init]] (calls)
- [[internal/gitctx/gittest/gittest.go/Write]] (calls)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
