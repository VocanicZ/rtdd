---
id: "internal/protocol/validate.go::validateMDC"
source: "internal/protocol/validate.go"
start: 141
end: 158
kind: "func"
is-test: false
tags: [rtdd/code]
---
# validateMDC

#rtdd/code

## Tests

- [[internal/protocol/validate_test.go/TestValidateMDCPassesOnItsOwnRender]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenFrontmatterStripped]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenGlobsMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenAlwaysApplyMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenRequiredSectionBodyMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenItSwallowsASkillOnlySection]] (calls)

## Callers

none

## Callees

- [[internal/protocol/validate.go/hasFrontmatter]] (calls)
- [[internal/protocol/validate.go/withinBudget]] (calls)
- [[internal/protocol/validate.go/requiredBodiesPresent]] (calls)
- [[internal/protocol/validate.go/noForeignSections]] (calls)
- [[internal/protocol/validate.go/requiredHeadingsPresent]] (calls)
