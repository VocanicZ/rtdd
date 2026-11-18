---
id: "bench/tests/test_session.py::_points"
source: "bench/tests/test_session.py"
start: 64
end: 69
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _points

#rtdd/code

## Tests

- [[bench/tests/test_session.py/test_drift_accumulates_the_changed_set_across_uncommitted_cycles]] (calls)
- [[bench/tests/test_session.py/test_drift_denominator_grows_with_the_tests_the_cycles_add]] (calls)
- [[bench/tests/test_session.py/test_drift_never_moves_head_or_commits]] (calls)
- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)
- [[bench/tests/test_session.py/test_default_selector_is_the_rtdd_binary_against_base_head]] (calls)
- [[bench/tests/test_session.py/test_run_drift_rejects_a_worktree_that_is_not_at_the_first_parent]] (calls)
- [[bench/tests/test_session.py/test_a_cycle_whose_tree_will_not_collect_publishes_no_ratio]] (calls)

## Callers

none

## Callees

- [[bench/replay/gitwork.py/ReplayPoint]] (calls)
