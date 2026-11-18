---
id: "internal/protocol/parse.go::Parse"
source: "internal/protocol/parse.go"
start: 144
end: 250
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Parse

#rtdd/code

## Tests

- [[cmd/rtdd-gen/main_test.go/TestVerifyCatchesAgentsSwallowingANonAgentsSection]] (calls)
- [[internal/graphbuild/overlay_test.go/TestOverlayLinksAStaleFilesCallsToItsOwnFileTypeFirst]] (calls)
- [[internal/protocol/golden_test.go/TestGolden]] (calls)
- [[internal/protocol/parse_test.go/TestParseReadsMetaAndSections]] (calls)
- [[internal/protocol/parse_test.go/TestForOrdersByOrderNotFilePosition]] (calls)
- [[internal/protocol/parse_test.go/TestForFiltersByTarget]] (calls)
- [[internal/protocol/parse_test.go/TestBodyForPrefersTheVariant]] (calls)
- [[internal/protocol/parse_test.go/TestTitleIsRequired]] (calls)
- [[internal/protocol/parse_test.go/TestMissingMetaIsAnError]] (calls)
- [[internal/protocol/parse_test.go/TestUnterminatedSectionIsAnError]] (calls)
- [[internal/protocol/parse_test.go/TestUnterminatedVariantIsAnError]] (calls)
- [[internal/protocol/parse_test.go/TestDuplicateIDIsAnError]] (calls)
- [[internal/protocol/parse_test.go/TestMalformedOrderIsAnError]] (calls)
- [[internal/protocol/parse_test.go/TestUnknownTargetIsAnError]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllReturnsErrOverBudgetNamingTheTarget]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllFailsWhenARequiredSectionIsMissing]] (calls)
- [[internal/protocol/render_test.go/TestRenderAllSucceedsOnTheRealProtocol]] (calls)
- [[internal/protocol/validate_test.go/TestValidateSkillFailsWhenItSwallowsAnMDCOnlySection]] (calls)
- [[internal/scan/rules_test.go/TestACallLinksToSameNamedDefinitionsOfItsOwnFileTypeFirst]] (calls)

## Callers

- [[cmd/rtdd-gen/main.go/loadDoc]] (calls)
- [[cmd/rtdd-gen/main.go/cmdRender]] (calls)
- [[cmd/rtdd-gen/main_test.go/wantRendered]] (calls)
- [[cmd/rtdd/skill.go/cmdSkillInstall]] (calls)
- [[cmd/rtdd/skill.go/cmdSkillUninstall]] (calls)
- [[internal/install/install.go/Files]] (calls)
- [[internal/protocol/global_test.go/renderReal]] (calls)
- [[internal/protocol/render_test.go/sampleDoc]] (calls)

## Callees

- [[internal/protocol/parse.go/attrs]] (calls)
- [[internal/protocol/parse.go/directive]] (calls)
