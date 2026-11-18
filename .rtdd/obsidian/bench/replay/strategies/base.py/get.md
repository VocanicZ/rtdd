---
id: "bench/replay/strategies/base.py::get"
source: "bench/replay/strategies/base.py"
start: 81
end: 84
kind: "func"
is-test: false
tags: [rtdd/code]
---
# get

#rtdd/code

## Tests

- [[bench/tests/test_strategy_base.py/test_register_and_get]] (calls)
- [[bench/tests/test_strategy_base.py/test_unknown_strategy_is_an_error]] (calls)
- [[bench/tests/test_strategy_base.py/test_needs_parent_state_marks_strategies_seeded_in_the_parent_tree]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_is_registered_under_the_id_the_plan_fixes]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_importgraph_is_registered_under_the_id_the_plan_fixes]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/test_the_lf_baseline_is_registered_under_its_plan_id]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_path_is_registered_under_the_id_the_plan_fixes]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_the_random_baseline_is_registered_and_needs_no_parent_state]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_both_arms_are_registered_and_rtdd_answers_first]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_the_rtdd_strategy_is_registered_under_its_plan_id]] (calls)
- [[bench/tests/test_strategy_testmon.py/test_the_testmon_baseline_is_registered_under_its_plan_id]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_the_xdist_baseline_is_registered_and_needs_no_parent_state]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_replay]] (calls)
- [[bench/replay/cli.py/_rebuild_repo]] (calls)
- [[bench/replay/replay.py/_prepare_state]] (calls)
- [[bench/replay/replay.py/_cached_selection]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/replay/strategies/base.py/expand_file_selectors]] (calls)
- [[bench/replay/strategies/importgraph.py/reverse_reachable]] (calls)

## Callees

none
