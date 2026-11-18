---
id: "bench/replay/strategies/base.py::Strategy::select"
source: "bench/replay/strategies/base.py"
start: 70
end: 70
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_full.py/test_full_preserves_the_collected_order_and_selects_only_known_ids]] (calls)
- [[bench/tests/test_strategy_full.py/test_empty_suite_still_escalates]] (calls)
- [[bench/tests/test_strategy_full.py/test_the_changed_set_does_not_narrow_the_control_arm]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_passes_no_tuning_flags]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_graph_over_the_synth_repo_selects_the_right_tests]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_non_python_change_is_invisible_to_the_static_graph]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_import_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_the_graph_passes_no_tuning_flags]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/test_selects_only_previously_failing_tests]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/test_empty_cache_degenerates_to_the_full_suite_and_is_an_escalation]] (calls)
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
- [[bench/tests/test_strategy_randomratio.py/test_matches_the_peer_selection_size]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_is_deterministic_for_a_given_seed_and_commit]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_a_different_seed_gives_a_different_sample_at_the_same_commit]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_the_two_variants_of_one_commit_are_sampled_independently]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_missing_peer_size_is_a_harness_error]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_a_peer_that_selected_more_than_the_suite_holds_is_clamped]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_a_peer_that_selected_nothing_samples_nothing]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_it_samples_real_ids_from_the_synthetic_repo]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_the_round_1_arm_selects_round_1_only]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_the_rounds_1_2_arm_is_the_union_without_double_counting]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_select_takes_round_1_expanded_to_collected_ids]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_select_passes_only_shipped_defaults]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_an_empty_round_1_is_an_empty_selection_not_an_escalation]] (calls)
- [[bench/tests/test_strategy_testmon.py/test_missing_database_is_an_escalation]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_xdist_selects_everything_but_carries_parallel_exec_args]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_xdist_selects_the_whole_collected_suite_of_the_synthetic_repo]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_the_xdist_selections_exec_args_reach_the_pytest_invocation]] (calls)

## Callers

- [[bench/replay/strategies/base.py/Strategy]] (method)

## Callees

none
