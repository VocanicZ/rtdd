---
id: "internal/protocol/validate.go::validateSkill"
source: "internal/protocol/validate.go"
start: 117
end: 134
kind: "func"
is-test: false
tags: [rtdd/code]
---
# validateSkill

#rtdd/code

## Tests

- [[internal/protocol/validate_test.go/TestValidateSkillPassesOnItsOwnRender]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenFrontmatterMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenRequiredSectionHeadingMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenRequiredSectionBodyMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenItSwallowsAnMDCOnlySection]] (calls)

## Callers

none

## Callees

- [[internal/protocol/testdata/golden/SKILL.md/_file]] (references)
- [[internal/protocol/validate.go/hasFrontmatter]] (calls)
- [[internal/protocol/validate.go/withinBudget]] (calls)
- [[internal/protocol/validate.go/requiredBodiesPresent]] (calls)
- [[internal/protocol/validate.go/noForeignSections]] (calls)
- [[internal/protocol/validate.go/requiredHeadingsPresent]] (calls)
