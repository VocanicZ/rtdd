---
id: "internal/install/migrate.go::PlanMigration"
source: "internal/install/migrate.go"
start: 25
end: 49
kind: "func"
is-test: false
tags: [rtdd/code]
---
# PlanMigration

#rtdd/code

## Tests

- [[internal/contract/plan_n3_test.go/TestN3PlanSettlesTheSharedDecisions]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesEveryV02Artefact]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationRemovesTheV01ReportAndTheAdapterOverride]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationDeletesAGitattributesHoldingOnlyTheV02Line]] (calls)
- [[internal/install/migrate_test.go/TestPlanMigrationOnAFreshRepositoryPlansNothing]] (calls)

## Callers

- [[internal/install/install.go/Plan]] (calls)

## Callees

- [[internal/install/uninstall.go/removeLine]] (calls)
