---
id: "internal/protocol/parse.go::HasTarget"
source: "internal/protocol/parse.go"
start: 69
end: 76
kind: "func"
is-test: false
tags: [rtdd/code]
---
# HasTarget

#rtdd/code

## Tests

- [[cmd/rtdd-gen/main_test.go/TestVerifyCatchesAgentsSwallowingANonAgentsSection]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenItSwallowsTheSkillBody]] (calls)
- [[internal/protocol/validate_test.go/TestValidateAgentsFailsWhenItSwallowsASharedSection]] (calls)
- [[internal/protocol/validate_test.go/TestValidateMDCFailsWhenItSwallowsASkillOnlySection]] (calls)

## Callers

- [[internal/protocol/parse.go/For]] (calls)
- [[internal/protocol/validate.go/noForeignSections]] (calls)

## Callees

none
