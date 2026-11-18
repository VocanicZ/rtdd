---
id: "bench/tests/test_strategy_base.py::_ctx"
source: "bench/tests/test_strategy_base.py"
start: 56
end: 70
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _ctx

#rtdd/code

## Tests

- [[bench/tests/test_strategy_base.py/test_register_and_get]] (calls)
- [[bench/tests/test_strategy_base.py/test_commit_context_carries_the_changed_set_and_the_full_test_list]] (calls)
- [[bench/tests/test_strategy_base.py/test_commit_context_peer_sizes_defaults_are_not_shared]] (calls)
- [[bench/tests/test_strategy_base.py/test_validate_selection_passes_a_selection_drawn_from_the_collected_tests]] (calls)
- [[bench/tests/test_strategy_base.py/test_validate_selection_rejects_a_test_id_the_repo_never_collected]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_whole_file_selector_expands_to_the_tests_that_file_collected]] (calls)
- [[bench/tests/test_strategy_base.py/test_expansion_does_not_duplicate_an_id_the_strategy_already_named]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_file_selector_that_collected_nothing_selects_nothing]] (calls)
- [[bench/tests/test_strategy_base.py/test_an_unknown_node_id_is_still_an_error]] (calls)
- [[bench/tests/test_strategy_base.py/test_a_test_the_base_tree_collected_but_the_child_removed_is_dropped_not_fatal]] (calls)
- [[bench/tests/test_strategy_base.py/test_an_id_neither_tree_ever_collected_is_still_an_error]] (calls)

## Callers

none

## Callees

- [[bench/replay/strategies/base.py/CommitContext]] (calls)
