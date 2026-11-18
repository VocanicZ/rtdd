---
id: "bench/replay/strategies/base.py::CommitContext"
source: "bench/replay/strategies/base.py"
start: 42
end: 61
kind: "class"
is-test: false
tags: [rtdd/code]
---
# CommitContext

#rtdd/code

## Tests

- [[bench/tests/test_strategy_full.py/test_full_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_graph_over_the_synth_repo_selects_the_right_tests]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_non_python_change_is_invisible_to_the_static_graph]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_import_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_the_graph_passes_no_tuning_flags]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_path_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/tests/test_strategy_base.py/_ctx]] (calls)
- [[bench/tests/test_strategy_full.py/_ctx]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/_ctx]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/_ctx]] (calls)
- [[bench/tests/test_strategy_randomratio.py/_ctx]] (calls)
- [[bench/tests/test_strategy_rounds.py/_ctx]] (calls)
- [[bench/tests/test_strategy_rtdd.py/_ctx]] (calls)
- [[bench/tests/test_strategy_testmon.py/_ctx]] (calls)
- [[bench/tests/test_strategy_xdist.py/_ctx]] (calls)

## Callees

- [[bench/replay/strategies/base.py/CommitContext/changed_paths]] (method)
