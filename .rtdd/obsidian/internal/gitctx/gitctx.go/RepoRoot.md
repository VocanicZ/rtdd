---
id: "internal/gitctx/gitctx.go::RepoRoot"
source: "internal/gitctx/gitctx.go"
start: 64
end: 74
kind: "func"
is-test: false
tags: [rtdd/code]
---
# RepoRoot

#rtdd/code

## Tests

- [[cmd/rtdd/which_perf_test.go/TestWhichOnThisRepositoryWithAWarmCacheIsUnder1s]] (calls)
- [[internal/contract/contract_test.go/TestInterfaceContractCarriesM1aAmendments]] (calls)
- [[internal/gitctx/gitctx_test.go/TestRepoRoot]] (calls)
- [[internal/gitctx/gitctx_test.go/TestRepoRootOutsideARepoIsAnError]] (calls)
- [[internal/graphbuild/perf_test.go/TestWarmBuildOfThisRepositoryIsUnder1s]] (calls)
- [[internal/graphbuild/selfscan_test.go/TestThisRepositoryHasNoKeywordOrBuiltinNamedNodes]] (calls)

## Callers

- [[cmd/rtdd/event.go/finish]] (calls)
- [[cmd/rtdd/graph.go/cmdGraph]] (calls)
- [[cmd/rtdd/graphenv.go/graphRoot]] (calls)
- [[internal/rounds/perf_test.go/thisRepositoryGraph]] (calls)

## Callees

- [[internal/gitctx/gitctx.go/git]] (calls)
