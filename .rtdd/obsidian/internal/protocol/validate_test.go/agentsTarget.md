---
id: "internal/protocol/validate_test.go::agentsTarget"
source: "internal/protocol/validate_test.go"
start: 18
end: 25
kind: "func"
is-test: false
tags: [rtdd/code]
---
# agentsTarget

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

- [[internal/protocol/targets.go/TargetByName]] (calls)
