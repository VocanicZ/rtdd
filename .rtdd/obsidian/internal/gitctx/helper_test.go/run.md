---
id: "internal/gitctx/helper_test.go::run"
source: "internal/gitctx/helper_test.go"
start: 29
end: 44
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestEveryRepoScriptIsExecutable]] (calls)
- [[internal/contract/plan_m6b_test.go/TestM6bPlanCoversEveryPRDAcceptanceCriterion]] (calls)
- [[internal/gitctx/helper_test.go/_file_]] (calls)
- [[internal/graphbuild/cache_test.go/TestWarmBuildKeepsImportsAndSupertypes]] (calls)
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

- [[cmd/rtdd/doctor.go/graphifyState]] (calls)
- [[cmd/rtdd/graph.go/cmdGraph]] (calls)
- [[cmd/rtdd/which.go/graphLine]] (calls)
- [[internal/gitctx/helper_test.go/gitRun]] (calls)
- [[internal/graphbuild/overlay_test.go/overlayGraphify]] (calls)

## Callees

none
