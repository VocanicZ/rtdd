---
id: "internal/protocol/render.go::RenderAll"
source: "internal/protocol/render.go"
start: 120
end: 141
kind: "func"
is-test: false
tags: [rtdd/code]
---
# RenderAll

#rtdd/code

## Tests

- [[internal/protocol/golden_test.go/TestGolden]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllKeysByOutPath]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllTargetsGetDifferentSections]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllIsByteStableAcrossCalls]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllReturnsErrOverBudgetNamingTheTarget]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllFailsWhenARequiredSectionIsMissing]] (calls)
- [[internal/protocol/render_test.go/TestRenderSkillFrontmatterAndGeneratedHeader]] (calls)
- [[internal/protocol/render_test.go/TestRenderMDCFrontmatterHasGlobsAndAlwaysApply]] (calls)
- [[internal/protocol/render_test.go/TestRenderAgentsHasBeginEndMarkers]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllSucceedsOnTheRealProtocol]] (calls)

## Callers

- [[cmd/rtdd-gen/main.go/cmdRender]] (calls)
- [[cmd/rtdd-gen/main.go/cmdCheck]] (calls)
- [[cmd/rtdd-gen/main_test.go/wantRendered]] (calls)
- [[internal/install/install.go/Files]] (calls)
- [[internal/protocol/global_test.go/renderReal]] (calls)

## Callees

- [[internal/protocol/render.go/requireSections]] (calls)
