---
id: "bench/replay/strategies/pathheuristic.py::PathHeuristic"
source: "bench/replay/strategies/pathheuristic.py"
start: 41
end: 67
kind: "class"
is-test: false
tags: [rtdd/code]
---
# PathHeuristic

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

none

## Callees

- [[bench/replay/strategies/pathheuristic.py/PathHeuristic/prepare]] (method)
- [[bench/replay/strategies/pathheuristic.py/PathHeuristic/select]] (method)
