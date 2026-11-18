---
id: "internal/protocol/validate.go::validateAgents"
source: "internal/protocol/validate.go"
start: 165
end: 187
kind: "func"
is-test: false
tags: [rtdd/code]
---
# validateAgents

#rtdd/code

## Tests

- [[internal/protocol/validate_test.go/TestValidateAgentsPassesOnItsOwnRender]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenItSwallowsTheSkillBody]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenOverBudgetWrapsErrOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenBeginMarkerMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenEndMarkerMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenEmpty]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenRequiredSectionContentMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenItSwallowsASharedSection]] (calls)

## Callers

none

## Callees

- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
- [[internal/protocol/validate.go/withinBudget]] (calls)
- [[internal/protocol/validate.go/requiredBodiesPresent]] (calls)
- [[internal/protocol/validate.go/noForeignSections]] (calls)
