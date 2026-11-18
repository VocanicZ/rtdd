---
id: "cmd/rtdd/changed.go::excludeRtddDir"
source: "cmd/rtdd/changed.go"
start: 37
end: 49
kind: "func"
is-test: false
tags: [rtdd/code]
---
# excludeRtddDir

#rtdd/code

## Tests

- [[cmd/rtdd/changed_test.go/TestExcludeRtddDirDropsRtddsOwnBookkeeping]] (calls)
- [[cmd/rtdd/changed_test.go/TestExcludeRtddDirKeepsEverythingElse]] (calls)
- [[cmd/rtdd/changed_test.go/TestExcludeRtddDirClearsAnOldPathUnderRtdd]] (calls)
- [[cmd/rtdd/changed_test.go/TestExcludeRtddDirDropsARenameWithinRtdd]] (calls)

## Callers

- [[cmd/rtdd/changed.go/changedSet]] (calls)

## Callees

- [[cmd/rtdd/changed.go/underRtddDir]] (calls)
