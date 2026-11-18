---
id: "bench/swebench/budget.py::Budget::restore"
source: "bench/swebench/budget.py"
start: 50
end: 60
kind: "method"
is-test: false
tags: [rtdd/code]
---
# restore

#rtdd/code

## Tests

- [[bench/swebench/tests/test_budget.py/test_restore_adopts_a_previous_runs_spend_so_a_resume_tops_it_up]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_carries_the_previous_runs_wall_clock]] (calls)
- [[bench/swebench/tests/test_budget.py/test_a_restored_budget_snapshots_the_whole_benchmarks_spend]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_of_an_empty_snapshot_changes_nothing]] (calls)
- [[bench/swebench/tests/test_budget.py/test_check_refuses_a_resume_that_is_already_over_the_cap]] (calls)

## Callers

- [[bench/swebench/budget.py/Budget]] (method)
- [[bench/swebench/run_arm.py/run_arm]] (calls)

## Callees

none
