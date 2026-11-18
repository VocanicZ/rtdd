---
id: "internal/protocol/render_test.go::sampleDoc"
source: "internal/protocol/render_test.go"
start: 56
end: 63
kind: "func"
is-test: false
tags: [rtdd/code]
---
# sampleDoc

#rtdd/code

## Tests

- [[internal/protocol/render_test.go/TestRenderAllKeysByOutPath]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllTargetsGetDifferentSections]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllIsByteStableAcrossCalls]] (calls)
- [[internal/protocol/render_test.go/TestRenderSkillFrontmatterAndGeneratedHeader]] (calls)
- [[internal/protocol/render_test.go/TestRenderMDCFrontmatterHasGlobsAndAlwaysApply]] (calls)
- [[internal/protocol/render_test.go/TestRenderAgentsHasBeginEndMarkers]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillPassesOnItsOwnRender]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenFrontmatterMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenRequiredSectionHeadingMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCPassesOnItsOwnRender]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenFrontmatterStripped]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenGlobsMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenAlwaysApplyMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsPassesOnItsOwnRender]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenItSwallowsTheSkillBody]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenOverBudgetWrapsErrOverBudget]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenBeginMarkerMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenEndMarkerMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenEmpty]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenRequiredSectionContentMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenItSwallowsASharedSection]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenRequiredSectionBodyMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenRequiredSectionBodyMissing]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenItSwallowsASkillOnlySection]] (calls)

## Callers

none

## Callees

- [[internal/protocol/parse.go/Parse]] (calls)
