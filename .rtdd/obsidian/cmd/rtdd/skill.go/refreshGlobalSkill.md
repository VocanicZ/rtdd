---
id: "cmd/rtdd/skill.go::refreshGlobalSkill"
source: "cmd/rtdd/skill.go"
start: 242
end: 286
kind: "func"
is-test: false
tags: [rtdd/code]
---
# refreshGlobalSkill

#rtdd/code

## Tests

- [[cmd/rtdd/agent_matrix_test.go/TestRefreshGlobalSkillSaysItRemovedTheOldCodexSkill]] (calls)
- [[cmd/rtdd/skill_test.go/TestRefreshGlobalSkillRewritesAnOlderReleasesSkill]] (calls)
- [[cmd/rtdd/skill_test.go/TestRefreshGlobalSkillIsSilentWhenNoAgentIsInstalled]] (calls)

## Callers

- [[cmd/rtdd/agent_matrix_test.go/rowUpdateRefreshes]] (calls)
- [[cmd/rtdd/init.go/cmdInit]] (calls)
- [[cmd/rtdd/skill.go/cmdSkillInstall]] (calls)
- [[cmd/rtdd/update.go/cmdUpdate]] (calls)

## Callees

- [[cmd/rtdd/skill.go/homeDir]] (calls)
- [[cmd/rtdd/skill.go/missingSteps]] (calls)
- [[internal/install/global.go/PlanGlobal]] (calls)
- [[internal/install/install.go/Apply]] (calls)
- [[internal/install/install.go/Files]] (calls)
