---
id: "internal/graphbuild/overlay_test.go::TestBuildBaseMakesFilesChangedAgainstItStale"
source: "internal/graphbuild/overlay_test.go"
start: 220
end: 244
kind: "func"
is-test: true
tags: [rtdd/test]
---
# TestBuildBaseMakesFilesChangedAgainstItStale

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[cmd/rtdd/testdata/which-golden/edit/src/calc.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/repo/src/calc.py/_file]] (references)
- [[internal/gitctx/gittest/gittest.go/Write]] (calls)
- [[internal/gitctx/gittest/gittest.go/Commit]] (calls)
- [[internal/graph/config.go/DefaultConfig]] (calls)
- [[internal/graphbuild/build.go/Build]] (calls)
- [[internal/graphbuild/build_test.go/repo]] (calls)
- [[internal/graphbuild/build_test.go/build]] (calls)
- [[internal/graphbuild/overlay_test.go/overlayGraphify]] (calls)
- [[internal/scan/testdata/python/calc.py/_file]] (references)
