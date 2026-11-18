---
id: "internal/install/link_test.go::installGlobal"
source: "internal/install/link_test.go"
start: 42
end: 52
kind: "func"
is-test: false
tags: [rtdd/code]
---
# installGlobal

#rtdd/code

## Tests

- [[internal/install/link_test.go/TestPlanGlobalLinksHermesToTheCanonicalSkill]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalWritesNoCanonicalCopyWhenNothingLinks]] (calls)
- [[internal/install/link_test.go/TestRefreshingTheCanonicalCopyRefreshesEveryLinkedAgent]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalReplacesAnEarlierCopyWithALink]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalLeavesAHandWrittenFileAtALinkedPathAlone]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalCopiesWhenTheCanonicalPathIsNotRtdds]] (calls)
- [[internal/install/link_test.go/TestPlanGlobalUninstallRemovesLinksAndTheCanonicalCopy]] (calls)

## Callers

none

## Callees

- [[internal/install/global.go/PlanGlobal]] (calls)
- [[internal/install/global_test.go/globalFiles]] (calls)
- [[internal/install/install.go/Apply]] (calls)
