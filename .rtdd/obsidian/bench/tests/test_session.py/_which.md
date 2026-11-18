---
id: "bench/tests/test_session.py::_which"
source: "bench/tests/test_session.py"
start: 35
end: 43
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _which

#rtdd/code

## Tests

- [[bench/tests/test_session.py/test_drift_denominator_grows_with_the_tests_the_cycles_add]] (calls)
- [[bench/tests/test_session.py/test_drift_never_moves_head_or_commits]] (calls)
- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)
- [[bench/tests/test_session.py/test_run_drift_rejects_a_worktree_that_is_not_at_the_first_parent]] (calls)

## Callers

- [[bench/tests/test_session.py/test_drift_accumulates_the_changed_set_across_uncommitted_cycles/fake_select]] (calls)
- [[bench/tests/test_session.py/test_default_selector_is_the_rtdd_binary_against_base_head/fake_which]] (calls)

## Callees

- [[bench/replay/rtddio.py/WhichResult]] (calls)
