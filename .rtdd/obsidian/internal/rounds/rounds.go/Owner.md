---
id: "internal/rounds/rounds.go::Owner"
source: "internal/rounds/rounds.go"
start: 107
end: 119
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Owner

#rtdd/code

## Tests

- [[internal/rounds/filenodes_test.go/TestTheFileNodeOwnsOnlyLinesInNoDefinition]] (calls)
- [[internal/rounds/rounds_test.go/TestOwnerIsTheInnermostNodeOfALine]] (calls)
- [[internal/rounds/rounds_test.go/TestOwnerOfTwoNodesWithOneSpanIsTheSameInEveryOrder]] (calls)

## Callers

- [[cmd/rtdd/explain.go/explainTargets]] (calls)

## Callees

- [[internal/rounds/rounds.go/outerFirst]] (calls)
