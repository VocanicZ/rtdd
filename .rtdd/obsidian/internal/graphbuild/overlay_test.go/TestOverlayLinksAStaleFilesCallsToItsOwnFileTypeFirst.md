---
id: "internal/graphbuild/overlay_test.go::TestOverlayLinksAStaleFilesCallsToItsOwnFileTypeFirst"
source: "internal/graphbuild/overlay_test.go"
start: 203
end: 215
kind: "func"
is-test: true
tags: [rtdd/test]
---
# TestOverlayLinksAStaleFilesCallsToItsOwnFileTypeFirst

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[cmd/rtdd-gen/main.go/_file]] (references)
- [[cmd/rtdd/main.go/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/edit/money/money.go/Parse]] (calls)
- [[cmd/rtdd/testdata/which-golden/repo/money/money.go/Parse]] (calls)
- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
- [[internal/graphbuild/overlay.go/Overlay]] (calls)
- [[internal/protocol/parse.go/Parse]] (calls)
- [[internal/scan/scan.go/ScanFile]] (calls)
