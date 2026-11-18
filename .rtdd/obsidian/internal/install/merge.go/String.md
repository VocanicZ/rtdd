---
id: "internal/install/merge.go::String"
source: "internal/install/merge.go"
start: 40
end: 66
kind: "func"
is-test: false
tags: [rtdd/code]
---
# String

#rtdd/code

## Tests

- [[cmd/rtdd/agent_matrix_test.go/TestRefreshGlobalSkillSaysItRemovedTheOldCodexSkill]] (calls)
- [[cmd/rtdd/legacy_test.go/TestRenderLegacyListSaysNoneOrOneLinePerStep]] (calls)
- [[cmd/rtdd/skill_test.go/TestRefreshGlobalSkillIsSilentWhenNoAgentIsInstalled]] (calls)
- [[internal/contract/contract_test.go/TestInterfaceContractCarriesM1aAmendments]] (calls)
- [[internal/contract/contract_test.go/TestInterfaceContractCarriesEveryM2Addition]] (calls)

## Callers

- [[cmd/rtdd/doctor.go/renderDoctor]] (calls)
- [[cmd/rtdd/event.go/recordInit]] (calls)
- [[cmd/rtdd/skill.go/RenderSkillPrompt]] (calls)
- [[cmd/rtdd/uninstall_test.go/runAsIs]] (calls)
- [[cmd/rtdd/update.go/parseUpdateFlags]] (calls)
- [[internal/install/config.go/DefaultConfig]] (calls)

## Callees

none
