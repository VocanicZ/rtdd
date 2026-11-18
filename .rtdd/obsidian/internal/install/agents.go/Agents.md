---
id: "internal/install/agents.go::Agents"
source: "internal/install/agents.go"
start: 38
end: 156
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Agents

#rtdd/code

## Tests

- [[cmd/rtdd/agent_matrix_test.go/TestAgentMatrixColumnsAreTheAgentsTable]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/TestAgentMatrixLinksAreTheAgentsTable]] (calls)
- [[internal/install/agents_test.go/TestAgentsTableInvariants]] (calls)

## Callers

- [[cmd/rtdd/agent_matrix_test.go/agentRow]] (calls)
- [[cmd/rtdd/agent_matrix_test.go/rowProjectOptIn]] (calls)
- [[internal/install/agents.go/AgentStatus]] (calls)
- [[internal/install/agents.go/projectSkillFiles]] (calls)
- [[internal/install/global.go/GlobalSurfaces]] (calls)
- [[internal/install/legacy.go/PlanGlobalLegacyFiles]] (calls)

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[internal/protocol/testdata/golden/AGENTS.md/_file]] (references)
- [[internal/protocol/testdata/golden/SKILL.md/_file]] (references)
- [[internal/protocol/testdata/golden/rtdd.mdc/_file]] (references)
