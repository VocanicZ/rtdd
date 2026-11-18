---
id: "internal/gitctx/history.go::CommitDistance"
source: "internal/gitctx/history.go"
start: 15
end: 34
kind: "func"
is-test: false
tags: [rtdd/code]
---
# CommitDistance

#rtdd/code

## Tests

- [[internal/gitctx/history_test.go/TestCommitDistance]] (calls)
- [[internal/gitctx/history_test.go/TestCommitDistanceUnreachableIsMinusOne]] (calls)
- [[internal/gitctx/history_test.go/TestCommitDistanceAfterAnAmendMakesTheOldShaUnreachable]] (calls)

## Callers

- [[internal/gitctx/history.go/Older]] (calls)

## Callees

- [[internal/gitctx/gitctx.go/git]] (calls)
- [[internal/gitctx/history.go/reachable]] (calls)
