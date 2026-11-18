---
id: "internal/testenv/testenv.go::SkipIfSaturated"
source: "internal/testenv/testenv.go"
start: 31
end: 36
kind: "func"
is-test: false
tags: [rtdd/code]
---
# SkipIfSaturated

#rtdd/code

## Tests

- [[cmd/rtdd/which_perf_test.go/TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s]] (calls)
- [[internal/graphbuild/perf_test.go/TestColdBuildOf10000FilesIsUnder10s]] (calls)
- [[internal/graphbuild/perf_test.go/TestWarmBuildOfThisRepositoryIsUnder1s]] (calls)
- [[internal/rounds/perf_test.go/TestRoundsOfEveryLineOfThisRepositoryIsUnder1s]] (calls)
- [[internal/testenv/testenv_test.go/TestSkipIfSaturatedSkipsUnderTheOverride]] (calls)

## Callers

none

## Callees

- [[internal/testenv/testenv.go/saturated]] (calls)
