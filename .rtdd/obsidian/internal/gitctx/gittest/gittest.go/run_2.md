---
id: "internal/gitctx/gittest/gittest.go::run"
source: "internal/gitctx/gittest/gittest.go"
start: 33
end: 47
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run

#rtdd/code

## Tests

- [[cmd/rtdd/doctor_graph_test.go/TestDoctorReportsTheGraphTestFilesAndOverLinkedNames]] (calls)
- [[internal/contract/contract_test.go/TestEveryRepoScriptIsExecutable]] (calls)
- [[internal/contract/plan_m6b_test.go/TestM6bPlanCoversEveryPRDAcceptanceCriterion]] (calls)
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

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[cmd/rtdd/uninstall_test.go/runAsIs]] (calls)
- [[internal/gitctx/gittest/gittest.go/Run]] (calls)
- [[internal/gitctx/gittest/gittest.go/InitRepo]] (calls)
- [[internal/graphbuild/overlay_test.go/overlayGraphify]] (calls)

## Callees

none
