---
id: "bench/swebench/budget.py::Budget::start"
source: "bench/swebench/budget.py"
start: 47
end: 48
kind: "method"
is-test: false
tags: [rtdd/code]
---
# start

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
- [[bench/swebench/tests/test_budget.py/test_check_refuses_a_resume_that_is_already_over_the_cap]] (calls)
- [[bench/swebench/tests/test_budget.py/test_check_is_silent_while_both_ceilings_hold]] (calls)

## Callers

- [[bench/swebench/budget.py/Budget]] (method)
- [[bench/swebench/run_arm.py/run_arm]] (calls)

## Callees

none
