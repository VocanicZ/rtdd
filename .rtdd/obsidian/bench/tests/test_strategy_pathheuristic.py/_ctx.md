---
id: "bench/tests/test_strategy_pathheuristic.py::_ctx"
source: "bench/tests/test_strategy_pathheuristic.py"
start: 31
end: 43
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _ctx

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

## Callers

none

## Callees

- [[bench/replay/gitwork.py/Change]] (calls)
- [[bench/replay/strategies/base.py/CommitContext]] (calls)
