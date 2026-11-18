---
id: "bench/swebench/run_arm.py"
source: "bench/swebench/run_arm.py"
start: 1
end: 355
kind: "file"
is-test: false
tags: [rtdd/file]
---
# run_arm.py

#rtdd/file

## Tests

- [[bench/swebench/tests/test_acceptance.py/test_every_arm_refuses_from_the_command_line_before_any_model_call]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_dry_run_cli_builds_five_arms_over_the_frozen_list_with_no_network]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_dry_run_prints_the_real_repository_root_the_same_for_every_arm]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_dry_run_needs_no_signed_pre_registration_so_ci_can_run_it_here]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_one_ci_run_covers_the_suite_the_refusal_the_dry_run_and_the_lint]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_docs_say_how_to_reproduce_the_harness_and_what_should_refuse]] (references)
- [[bench/tests/test_coldcache.py/test_the_driver_root_follows_HOME_and_not_the_variable]] (references)
- [[bench/tests/test_coldcache.py/test_the_driver_root_matches_the_literal_run_arm_actually_uses]] (references)
- [[internal/contract/cold_cache_gate_test.go/TestColdCacheGateRunsEveryBenchCommandTheWorkflowRuns]] (references)
- [[internal/contract/cold_cache_gate_test.go/TestColdCacheGateProvesColdnessBeforeItRunsAnyBenchCommand]] (references)
- [[internal/contract/cold_cache_gate_test.go/TestColdCacheGateRefusesToPublishOrToDisturbTheWarmStore]] (references)

## Callers

- [[.github/workflows/ci.yml/_file]] (references)
- [[DEVELOPMENT.md/_file]] (references)
- [[bench/replay/coldcache.py/driver_root]] (references)
- [[docs/bench/swebench-harness.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/12-bench-rounds.md/_file]] (references)
- [[scripts/ci-cold-cache.sh/_file]] (references)
- [[scripts/ci-prereg.sh/_file]] (references)

## Callees

none
