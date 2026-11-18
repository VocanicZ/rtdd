---
id: "cmd/rtdd/which_json_test.go::goldenWhichRepo"
source: "cmd/rtdd/which_json_test.go"
start: 38
end: 52
kind: "func"
is-test: false
tags: [rtdd/code]
---
# goldenWhichRepo

#rtdd/code

## Tests

- [[cmd/rtdd/event_test.go/TestEventLogWhichWritesOneLineWithTheJSONCounts]] (calls)
- [[cmd/rtdd/event_test.go/TestEventLogUnsetOrUnwritableChangesNothing]] (calls)
- [[cmd/rtdd/event_test.go/TestEventLogParallelProcessesWriteWholeLines]] (calls)
- [[cmd/rtdd/which_json_test.go/TestWhichJSONGolden]] (calls)

## Callers

none

## Callees

- [[cmd/rtdd/testdata/which-golden/repo/src/legacy.py/_file]] (references)
- [[cmd/rtdd/which_json_test.go/copyTree]] (calls)
- [[internal/gitctx/gittest/gittest.go/Init]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
