---
id: "internal/protocol/parse.go::For"
source: "internal/protocol/parse.go"
start: 78
end: 92
kind: "func"
is-test: false
tags: [rtdd/code]
---
# For

#rtdd/code

## Tests

- [[internal/protocol/parse_test.go/TestForOrdersByOrderNotFilePosition]] (calls)
- [[internal/protocol/parse_test.go/TestForFiltersByTarget]] (calls)
- [[internal/protocol/parse_test.go/TestBodyForPrefersTheVariant]] (calls)

## Callers

- [[internal/protocol/render.go/requireSections]] (calls)
- [[internal/protocol/render.go/renderSkill]] (calls)
- [[internal/protocol/render.go/renderAgents]] (calls)
- [[internal/protocol/render.go/renderMDC]] (calls)
- [[internal/protocol/validate.go/requiredBodiesPresent]] (calls)
- [[internal/protocol/validate.go/requiredHeadingsPresent]] (calls)
- [[internal/protocol/validate_test.go/guttedBodies]] (calls)

## Callees

- [[internal/protocol/parse.go/HasTarget]] (calls)
