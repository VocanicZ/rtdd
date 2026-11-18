---
id: "bench/swebench/budget.py::Budget::add"
source: "bench/swebench/budget.py"
start: 75
end: 79
kind: "method"
is-test: false
tags: [rtdd/code]
---
# add

#rtdd/code

## Tests

- [[bench/swebench/tests/test_budget.py/test_usd_is_the_two_per_million_prices_applied_to_the_two_counts]] (calls)
- [[bench/swebench/tests/test_budget.py/test_add_accumulates_across_calls]] (calls)
- [[bench/swebench/tests/test_budget.py/test_add_raises_when_the_spend_cap_is_crossed]] (calls)
- [[bench/swebench/tests/test_budget.py/test_the_tokens_that_crossed_the_cap_are_still_counted]] (calls)
- [[bench/swebench/tests/test_budget.py/test_add_raises_when_the_wall_clock_cap_is_crossed]] (calls)
- [[bench/swebench/tests/test_budget.py/test_snapshot_records_the_assumed_prices_beside_the_actual_counts]] (calls)
- [[bench/swebench/tests/test_budget.py/test_snapshot_is_json_serialisable]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_adopts_a_previous_runs_spend_so_a_resume_tops_it_up]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_carries_the_previous_runs_wall_clock]] (calls)
- [[bench/swebench/tests/test_budget.py/test_a_restored_budget_snapshots_the_whole_benchmarks_spend]] (calls)
- [[bench/swebench/tests/test_budget.py/test_check_is_silent_while_both_ceilings_hold]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_probe_keeps_child_tests_over_parent_source]] (calls)
- [[bench/tests/test_rtddio.py/test_real_binary_honours_the_consumed_schema]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_verified_against_the_synthetic_repo]] (calls)

## Callers

- [[bench/swebench/budget.py/Budget]] (method)
- [[bench/swebench/run_arm.py/run_arm]] (calls)
- [[bench/tests/synthrepo.py/build_synth_repo]] (calls)

## Callees

- [[bench/swebench/budget.py/Budget/check]] (calls)
