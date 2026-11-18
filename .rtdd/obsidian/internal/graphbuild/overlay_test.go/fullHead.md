---
id: "internal/graphbuild/overlay_test.go::fullHead"
source: "internal/graphbuild/overlay_test.go"
start: 62
end: 64
kind: "func"
is-test: false
tags: [rtdd/code]
---
# fullHead

#rtdd/code

## Tests

- [[internal/graphbuild/ignore_test.go/TestTooStaleKeepsTheStaleSetItMeasured]] (calls)
- [[internal/graphbuild/ignore_test.go/TestExactlyHalfStaleKeepsGraphify]] (calls)
- [[internal/graphbuild/ignore_test.go/TestGraphifyWithNoCodeFilesIsTooStale]] (calls)
- [[internal/graphbuild/ignore_test.go/TestMaxStaleRatioOverride]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)
- [[internal/graphbuild/scanset_test.go/TestTooStaleCountsCodeFilesOnly]] (calls)

## Callers

- [[internal/graphbuild/overlay_test.go/overlayGraphify]] (calls)
- [[internal/graphbuild/scanset_test.go/pyGraphify]] (calls)

## Callees

- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
