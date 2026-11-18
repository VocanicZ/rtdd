---
id: "bench/replay/strategies/base.py::Selection"
source: "bench/replay/strategies/base.py"
start: 30
end: 38
kind: "class"
is-test: false
tags: [rtdd/code]
---
# Selection

#rtdd/code

## Tests

- [[bench/tests/test_strategy_base.py/test_selection_defaults]] (calls)
- [[bench/tests/test_strategy_base.py/test_selection_is_frozen_and_always_records_escalation]] (calls)
- [[bench/tests/test_strategy_base.py/test_validate_selection_passes_a_selection_drawn_from_the_collected_tests]] (calls)
- [[bench/tests/test_strategy_base.py/test_validate_selection_rejects_a_test_id_the_repo_never_collected]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_whole_file_selector_expands_to_the_tests_that_file_collected]] (calls)
- [[bench/tests/test_strategy_base.py/test_expansion_does_not_duplicate_an_id_the_strategy_already_named]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_file_selector_that_collected_nothing_selects_nothing]] (calls)
- [[bench/tests/test_strategy_base.py/test_an_unknown_node_id_is_still_an_error]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_test_the_base_tree_collected_but_the_child_removed_is_dropped_not_fatal]] (calls)
- [[bench/tests/test_strategy_base.py/test_an_id_neither_tree_ever_collected_is_still_an_error]] (calls)

## Callers

- [[bench/replay/replay.py/_cached_selection]] (calls)
- [[bench/replay/strategies/full.py/FullSuite/select]] (calls)
- [[bench/replay/strategies/importgraph.py/ImportGraph/select]] (calls)
- [[bench/replay/strategies/lastfailed.py/LastFailed/select]] (calls)
- [[bench/replay/strategies/pathheuristic.py/PathHeuristic/select]] (calls)
- [[bench/replay/strategies/randomratio.py/RandomRatio/select]] (calls)
- [[bench/replay/strategies/rtdd.py/Rtdd/select]] (calls)
- [[bench/replay/strategies/testmon.py/Testmon/select]] (calls)
- [[bench/replay/strategies/xdist.py/Xdist/select]] (calls)
- [[bench/tests/test_strategy_base.py/_Dummy/select]] (calls)

## Callees

none
