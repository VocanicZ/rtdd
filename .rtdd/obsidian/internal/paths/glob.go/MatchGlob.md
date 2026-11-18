---
id: "internal/paths/glob.go::MatchGlob"
source: "internal/paths/glob.go"
start: 36
end: 44
kind: "func"
is-test: false
tags: [rtdd/code]
---
# MatchGlob

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestInterfaceContractCarriesM1aAmendments]] (calls)
- [[internal/paths/glob_test.go/TestMatchGlob]] (calls)
- [[internal/paths/glob_test.go/TestMatchGlobDoesNotSwallowABadPattern]] (calls)

## Callers

- [[internal/graph/classify.go/matchAny]] (calls)
- [[internal/graphbuild/build.go/matchAny]] (calls)
- [[internal/scan/filter.go/matchAny]] (calls)

## Callees

- [[internal/paths/glob.go/ValidateGlob]] (calls)
- [[internal/paths/glob.go/matchSegments]] (calls)
