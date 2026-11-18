---
id: "scripts/ci-local.sh"
source: "scripts/ci-local.sh"
start: 1
end: 145
kind: "file"
is-test: false
tags: [rtdd/file]
---
# ci-local.sh

#rtdd/file

## Tests

- [[bench/tests/test_results_corpus_coverage.py/test_the_local_ci_gate_runs_the_bench_replay_suite]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIWorkflowIsRecordedActive]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIWorkflowStillTriggersOnPushToMainAndOnPullRequests]] (references)
- [[internal/contract/ci_workflow_test.go/TestCITestJobConcludedSuccessOnAMainCommit]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIBenchStepsRanWithoutAPrimedCache]] (references)
- [[internal/contract/ci_workflow_test.go/TestCIWorkflowPrimesNoBenchCache]] (references)
- [[internal/contract/ci_workflow_test.go/TestBothGatesRunTheReleaseSnapshotAndItsTests]] (references)
- [[internal/contract/ci_workflow_test.go/TestREADMEBaselineComparisonPageExists]] (references)
- [[internal/contract/ci_workflow_test.go/TestLocalCIPutsTheCheckoutsRtddFirstOnPathBeforeTheBenchGates]] (references)
- [[internal/contract/ci_workflow_test.go/TestLocalCIPutsTheCheckoutsRtddFirstOnPathBeforeTheBenchGates]] (references)
- [[internal/contract/contract_test.go/TestLocalCIEntrypointIsExecutableAndRunsTheSameChecks]] (references)
- [[internal/contract/plan_everyagent_test.go/TestEveryAgentPlanGlobalConstraintsCarryThePRDConstraints]] (references)
- [[internal/contract/plan_m7_test.go/TestM7PlanStepTwoNamesCommandAndExpectedFailure]] (references)
- [[internal/contract/plan_n4_test.go/TestN4PlanGlobalConstraintsCarryThePRDConstraints]] (references)
- [[internal/installtest/release_preflight_test.go/TestLinkageCheckEntrypointsNameTheTestsRealPackage]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[docs/bench/swebench-harness.md/_file]] (references)
- [[docs/plans/06-m6a-adapter-contract-v2.md/_file]] (references)
- [[docs/plans/06-m6b-static-tier.md/_file]] (references)
- [[docs/plans/06-m6c-junit-report.md/_file]] (references)
- [[docs/plans/06-m6d-shipped-adapters.md/_file]] (references)
- [[docs/plans/06-m6e-evidence-and-honesty.md/_file]] (references)
- [[docs/plans/07-m7-release-readiness.md/_file]] (references)
- [[docs/plans/08-one-pipeline.md/_file]] (references)
- [[docs/plans/09-node-graph-core.md/_file]] (references)
- [[docs/plans/10-rounds-cutover.md/_file]] (references)
- [[docs/plans/11-skill-install.md/_file]] (references)
- [[docs/plans/12-bench-rounds.md/_file]] (references)
- [[docs/plans/2026-10-24-every-agent.md/_file]] (references)

## Callees

- [[internal/install/protocol.md/_file]] (references)
- [[protocol/PROTOCOL.md/_file]] (references)
- [[scripts/ci-prereg.sh/_file]] (references)
- [[scripts/release-snapshot.sh/_file]] (references)
