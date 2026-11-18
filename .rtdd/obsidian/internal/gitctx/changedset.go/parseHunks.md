---
id: "internal/gitctx/changedset.go::parseHunks"
source: "internal/gitctx/changedset.go"
start: 221
end: 260
kind: "func"
is-test: false
tags: [rtdd/code]
---
# parseHunks

#rtdd/code

## Tests

- [[internal/gitctx/parsehunks_test.go/TestParseHunksPropagatesScannerError]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestParseHunksOnAWellFormedDiff]] (calls)

## Callers

- [[internal/gitctx/changedset.go/ChangedSet]] (calls)

## Callees

- [[internal/gitctx/changedset.go/stripDiffPrefix]] (calls)
- [[internal/gitctx/changedset.go/unquotePath]] (calls)
- [[internal/gitctx/changedset.go/parseHunkHeader]] (calls)
