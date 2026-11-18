---
id: "bench/replay/session.py::run_drift"
source: "bench/replay/session.py"
start: 88
end: 141
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run_drift

#rtdd/code

## Tests

- [[bench/tests/test_session.py/test_drift_accumulates_the_changed_set_across_uncommitted_cycles]] (calls)
- [[bench/tests/test_session.py/test_drift_denominator_grows_with_the_tests_the_cycles_add]] (calls)
- [[bench/tests/test_session.py/test_drift_never_moves_head_or_commits]] (calls)
- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)
- [[bench/tests/test_session.py/test_no_points_is_an_empty_curve_and_never_collects]] (calls)
- [[bench/tests/test_session.py/test_default_selector_is_the_rtdd_binary_against_base_head]] (calls)
- [[bench/tests/test_session.py/test_run_drift_rejects_a_worktree_that_is_not_at_the_first_parent]] (calls)
- [[bench/tests/test_session.py/test_a_cycle_whose_tree_will_not_collect_publishes_no_ratio]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_session]] (calls)

## Callees

- [[bench/replay/gitwork.py/git]] (calls)
- [[bench/replay/gitwork.py/materialise_natural]] (calls)
- [[bench/replay/gitwork.py/working_changed_paths]] (calls)
- [[bench/replay/rtddio.py/which]] (calls)
- [[bench/replay/runner.py/collect]] (calls)
- [[bench/replay/session.py/DriftPoint]] (calls)
- [[bench/replay/session.py/DriftCurve]] (calls)
- [[bench/swebench/prompts.py/build]] (calls)
- [[bench/swebench/tests/conftest.py/make_repo/build]] (calls)
- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once/build]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed/build]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit/build]] (calls)
