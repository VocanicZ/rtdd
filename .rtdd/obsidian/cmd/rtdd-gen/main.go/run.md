---
id: "cmd/rtdd-gen/main.go::run"
source: "cmd/rtdd-gen/main.go"
start: 39
end: 55
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestEveryRepoScriptIsExecutable]] (calls)
- [[internal/contract/plan_m6b_test.go/TestM6bPlanCoversEveryPRDAcceptanceCriterion]] (calls)
- [[internal/scan/refs_test.go/TestScanProseReadsPathsAndNoDefinition]] (calls)
- [[internal/scan/resolve_test.go/TestAReceiversDeclaredTypePicksItsClassesMethod]] (calls)
- [[internal/scan/resolve_test.go/TestACallPrefersTheDefinitionInAnImportedFile]] (calls)
- [[internal/scan/resolve_test.go/TestACallMatchingTooManyDefinitionsLinksToNoneAndIsCounted]] (calls)
- [[internal/scan/resolve_test.go/TestADeclaredReceiverTypeLinksOnlyWithinItsClassHierarchy]] (calls)
- [[internal/scan/resolve_test.go/TestADeclaredReceiverTypeImportedFromAFileLinksOnlyIntoIt]] (calls)
- [[internal/scan/resolve_test.go/TestReceiverOfReadsEachMemberCallForm]] (calls)
- [[internal/scan/resolve_test.go/TestResolutionEdgeCases]] (calls)
- [[internal/scan/rules_test.go/TestEverySameNamedDefinitionIsCalledButNotFromTheDefinitionLine]] (calls)
- [[internal/scan/rules_test.go/TestAnOpenerTheFileNeverClosesBlanksOnlyItself]] (calls)
- [[internal/scan/shapes_t2_test.go/TestTypeScriptClassMembersAreMethods]] (calls)
- [[internal/scan/shapes_t2_test.go/TestJavaMethodsButNotDeclarationsOrControlStatements]] (calls)

## Callers

- [[cmd/rtdd-gen/main.go/cmdCheck]] (calls)
- [[cmd/rtdd-gen/main_test.go/rtddgen]] (calls)

## Callees

- [[cmd/rtdd-gen/main.go/cmdRender]] (calls)
- [[cmd/rtdd-gen/main.go/cmdCheck]] (calls)
- [[cmd/rtdd-gen/main.go/cmdVerify]] (calls)
