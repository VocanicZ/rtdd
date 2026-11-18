---
id: "internal/installtest/release_artifacts_test.go::verifyArtifactLinkage"
source: "internal/installtest/release_artifacts_test.go"
start: 184
end: 213
kind: "func"
is-test: false
tags: [rtdd/code]
---
# verifyArtifactLinkage

#rtdd/code

## Tests

- [[internal/installtest/release_artifacts_test.go/TestReleaseArtifactsAreStaticallyLinked]] (calls)
- [[internal/installtest/release_artifacts_test.go/TestReleaseLinkageCheckRejectsADynamicallyLinkedBinary]] (calls)
- [[internal/installtest/release_artifacts_test.go/TestReleaseLinkageCheckRejectsAMismatchedArtifact]] (calls)

## Callers

none

## Callees

- [[internal/installtest/release_artifacts_test.go/verifyELFIsStaticallyLinked]] (calls)
- [[internal/installtest/release_artifacts_test.go/verifyMachOUsesOnlyBaseSystemDylibs]] (calls)
- [[internal/installtest/release_artifacts_test.go/verifyPEUsesOnlyBaseSystemDLLs]] (calls)
