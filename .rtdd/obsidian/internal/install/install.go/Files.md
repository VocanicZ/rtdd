---
id: "internal/install/install.go::Files"
source: "internal/install/install.go"
start: 267
end: 273
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Files

#rtdd/code

## Tests

- [[cmd/rtdd/init_gate_test.go/TestInitInADetectableRepoInstallsTheUnmodifiedSkill]] (calls)
- [[cmd/rtdd/init_migrate_test.go/TestInitMigratesAV02Repository]] (calls)
- [[cmd/rtdd/readme_agents_test.go/TestEverySkillFrontmatterFitsEveryAgent]] (calls)
- [[cmd/rtdd/update_refresh_test.go/TestUpdateFromV030InstallsTheSkillForAnAgentTheNewReleaseSupports]] (calls)
- [[cmd/rtdd/which_json_test.go/TestWhichJSONRoundFilesAreTheDeduplicatedTestFiles]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs]] (calls)
- [[internal/install/agents_test.go/TestAgentsTableInvariants]] (calls)
- [[internal/install/install_test.go/TestFilesReturnsTheEmbeddedGeneratedFrontEnds]] (calls)

## Callers

- [[cmd/rtdd/agent_matrix_test.go/rowUpdateRefreshes]] (calls)
- [[cmd/rtdd/init.go/cmdInit]] (calls)
- [[cmd/rtdd/skill.go/cmdSkillInstall]] (calls)
- [[cmd/rtdd/skill.go/cmdSkillPrompt]] (calls)
- [[cmd/rtdd/skill.go/refreshGlobalSkill]] (calls)
- [[cmd/rtdd/which_swarm_test.go/selectedFiles]] (calls)

## Callees

- [[internal/protocol/parse.go/Parse]] (calls)
- [[internal/protocol/render.go/RenderAll]] (calls)
