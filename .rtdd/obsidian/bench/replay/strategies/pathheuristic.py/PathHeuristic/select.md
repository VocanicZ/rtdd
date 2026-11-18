---
id: "bench/replay/strategies/pathheuristic.py::PathHeuristic::select"
source: "bench/replay/strategies/pathheuristic.py"
start: 48
end: 67
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_pathheuristic.py/test_source_change_maps_to_its_sibling_test_file]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_multi_file_change_unions]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_changed_test_file_selects_itself]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_changed_test_file_the_repo_never_collected_contributes_nothing]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_unmatched_change_selects_nothing_and_is_not_an_escalation]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_the_heuristic_does_no_package_matching]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_a_changed_package_init_reaches_nothing]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_non_python_changes_are_ignored]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_selection_only_ever_names_collected_test_ids]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_the_heuristic_passes_no_tuning_flags]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_path_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)

## Callers

- [[bench/replay/strategies/pathheuristic.py/PathHeuristic]] (method)

## Callees

- [[bench/replay/gitwork.py/is_test_path]] (calls)
- [[bench/replay/strategies/base.py/Selection]] (calls)
- [[bench/replay/strategies/pathheuristic.py/_file_of]] (calls)
- [[bench/replay/strategies/pathheuristic.py/candidate_test_files]] (calls)
- [[bench/replay/strategies/pathheuristic.py/tests_in_files]] (calls)
