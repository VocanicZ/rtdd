---
id: "bench/replay/gitwork.py::Change"
source: "bench/replay/gitwork.py"
start: 42
end: 46
kind: "class"
is-test: false
tags: [rtdd/code]
---
# Change

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_change_is_a_frozen_value]] (calls)
- [[bench/tests/test_strategy_base.py/test_commit_context_carries_the_changed_set_and_the_full_test_list]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_graph_over_the_synth_repo_selects_the_right_tests]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_non_python_change_is_invisible_to_the_static_graph]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_the_graph_passes_no_tuning_flags]] (calls)

## Callers

- [[bench/replay/gitwork.py/diff_changes]] (calls)
- [[bench/replay/gitwork.py/materialise_probe]] (calls)
- [[bench/replay/gitwork.py/working_changed_paths]] (calls)
- [[bench/tests/test_strategy_full.py/_ctx]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/_ctx]] (calls)
- [[bench/tests/test_strategy_rtdd.py/_ctx]] (calls)
- [[bench/tests/test_strategy_testmon.py/_ctx]] (calls)

## Callees

none
