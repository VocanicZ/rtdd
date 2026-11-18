---
id: "internal/install/config.go::DefaultConfig"
source: "internal/install/config.go"
start: 14
end: 34
kind: "func"
is-test: false
tags: [rtdd/code]
---
# DefaultConfig

#rtdd/code

## Tests

- [[cmd/rtdd/which_fanout_test.go/TestWhichNeverListsALogAsATest]] (calls)
- [[internal/contract/plan_n3_test.go/TestN3PlanSettlesTheSharedDecisions]] (calls)
- [[internal/install/config_test.go/TestDefaultConfigLoadsAsTheGraphDefaults]] (calls)
- [[internal/install/config_test.go/TestDefaultConfigNamesEveryKeyAndNoV02Key]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationOnAFreshRepositoryPlansNothing]] (calls)
- [[internal/install/migrate_test.go/TestPlanReplacesAV02OnlyConfigAndKeepsAV030One]] (calls)
- [[internal/install/plan_v030_test.go/TestPlanWritesTheConfigAndIgnoresTheGraphCache]] (calls)

## Callers

- [[internal/install/install.go/Plan]] (calls)

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[docs/specs/2026-10-07-node-graph.md/_file]] (references)
- [[internal/graph/config.go/DefaultConfig]] (calls)
- [[internal/install/merge.go/String]] (calls)
