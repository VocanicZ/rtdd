---
id: "internal/graphbuild/overlay_test.go::overlayGraphify"
source: "internal/graphbuild/overlay_test.go"
start: 74
end: 85
kind: "func"
is-test: false
tags: [rtdd/code]
---
# overlayGraphify

#rtdd/code

## Tests

- [[internal/graphbuild/cache_test.go/TestCacheServesGraphifysCommittedStaleFiles]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayDropsAnEdgeIntoARenamedNode]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildBaseMakesFilesChangedAgainstItStale]] (calls)
- [[internal/graphbuild/overlay_test.go/TestBuildRescanMakesTheNamedFileStale]] (calls)

## Callers

none

## Callees

- [[cmd/rtdd/testdata/which-golden/edit/src/calc.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/repo/src/calc.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/repo/tests/test_calc.py/_file]] (references)
- [[internal/gitctx/gittest/gittest.go/run_2]] (calls)
- [[internal/gitctx/helper_test.go/run]] (calls)
- [[internal/graphbuild/overlay_test.go/writeGraphify]] (calls)
- [[internal/graphbuild/overlay_test.go/fullHead]] (calls)
- [[internal/scan/testdata/python/calc.py/_file]] (references)
- [[internal/scan/testdata/python/test_calc.py/_file]] (references)
