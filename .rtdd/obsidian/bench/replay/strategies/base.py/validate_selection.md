---
id: "bench/replay/strategies/base.py::validate_selection"
source: "bench/replay/strategies/base.py"
start: 116
end: 145
kind: "func"
is-test: false
tags: [rtdd/code]
---
# validate_selection

#rtdd/code

## Tests

- [[bench/tests/test_strategy_base.py/test_validate_selection_passes_a_selection_drawn_from_the_collected_tests]] (calls)
- [[bench/tests/test_strategy_base.py/test_validate_selection_rejects_a_test_id_the_repo_never_collected]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_whole_file_selector_expands_to_the_tests_that_file_collected]] (calls)
- [[bench/tests/test_strategy_base.py/test_expansion_does_not_duplicate_an_id_the_strategy_already_named]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_file_selector_that_collected_nothing_selects_nothing]] (calls)
- [[bench/tests/test_strategy_base.py/test_an_unknown_node_id_is_still_an_error]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_test_the_base_tree_collected_but_the_child_removed_is_dropped_not_fatal]] (calls)
- [[bench/tests/test_strategy_base.py/test_an_id_neither_tree_ever_collected_is_still_an_error]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_preserves_the_collected_order_and_selects_only_known_ids]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_graph_over_the_synth_repo_selects_the_right_tests]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_import_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_selection_only_ever_names_collected_test_ids]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_path_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_it_samples_real_ids_from_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/strategies/base.py/UnknownTestError]] (calls)
- [[bench/replay/strategies/base.py/expand_file_selectors]] (calls)
