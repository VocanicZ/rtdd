---
id: "bench/replay/cache.py::Cache"
source: "bench/replay/cache.py"
start: 35
end: 137
kind: "class"
is-test: false
tags: [rtdd/code]
---
# Cache

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_key_includes_config_digest_so_a_config_change_misses]] (calls)
- [[bench/tests/test_cache.py/test_key_distinguishes_every_coordinate_of_the_tuple]] (calls)
- [[bench/tests/test_cache.py/test_key_is_not_confusable_by_concatenating_adjacent_parts]] (calls)
- [[bench/tests/test_cache.py/test_a_config_change_is_a_miss_not_a_silent_mix_of_old_and_new_numbers]] (calls)
- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once]] (calls)
- [[bench/tests/test_cache.py/test_json_survives_a_fresh_cache_object_on_the_same_root]] (calls)
- [[bench/tests/test_cache.py/test_artifact_round_trip]] (calls)
- [[bench/tests/test_cache.py/test_a_restored_artifact_is_byte_identical_to_what_was_stored]] (calls)
- [[bench/tests/test_cache.py/test_restoring_a_missing_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_artifact_hit_and_miss_are_counted]] (calls)
- [[bench/tests/test_cache.py/test_storing_an_artifact_twice_replaces_the_old_bytes]] (calls)
- [[bench/tests/test_cache.py/test_a_corrupt_json_entry_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_truncated_or_wrong_shaped_json_entry_is_a_miss]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_replaced_by_the_next_store]] (calls)
- [[bench/tests/test_cache.py/test_a_leftover_staging_directory_is_never_mistaken_for_an_entry]] (calls)
- [[bench/tests/test_cache.py/test_the_cache_writes_only_under_its_own_root]] (calls)
- [[bench/tests/test_replay.py/test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth]] (calls)
- [[bench/tests/test_replay.py/test_f_full_subtracts_the_failures_already_red_at_the_clean_parent]] (calls)
- [[bench/tests/test_replay.py/test_the_two_populations_are_tagged_and_never_pooled]] (calls)
- [[bench/tests/test_replay.py/test_every_cycle_produces_a_strategy_record_for_every_strategy]] (calls)
- [[bench/tests/test_replay.py/test_a_commit_that_collects_nothing_is_skipped_and_the_replay_continues]] (calls)
- [[bench/tests/test_replay.py/test_wall_clock_captures_all_three_forms_and_the_isolation_check]] (calls)
- [[bench/tests/test_replay.py/test_wall_clock_is_suppressed_on_ci]] (calls)
- [[bench/tests/test_replay.py/test_a_cached_run_replayed_under_different_hardware_does_not_publish_borrowed_timing]] (calls)
- [[bench/tests/test_replay.py/test_a_repo_outside_the_frozen_corpus_is_refused]] (calls)
- [[bench/tests/test_replay.py/test_a_second_replay_is_served_from_cache]] (calls)
- [[bench/tests/test_replay.py/test_a_config_change_is_a_cache_miss_not_a_silent_mix]] (calls)
- [[bench/tests/test_replay.py/test_f_full_excludes_a_red_that_outlives_the_parent]] (calls)
- [[bench/tests/test_replay.py/test_state_a_strategy_built_in_the_base_tree_is_not_reported_as_a_change]] (calls)
- [[bench/tests/test_replay.py/test_the_orchestrator_runs_the_suite_in_the_provisioned_interpreter]] (calls)
- [[bench/tests/test_replay.py/test_the_materialised_worktree_wins_the_import_over_the_editable_clone]] (calls)
- [[bench/tests/test_replay.py/test_a_base_tree_that_collects_nothing_is_skipped_not_fatal]] (calls)
- [[bench/tests/test_replay.py/test_a_second_replay_reproduces_the_records_byte_for_byte]] (calls)
- [[bench/tests/test_replay.py/test_the_parallel_baselines_subset_runs_are_actually_parallel]] (calls)
- [[bench/tests/test_replay.py/test_a_base_tree_rtdd_cannot_graph_is_skipped_not_fatal]] (calls)
- [[bench/tests/test_replay.py/test_the_replay_runs_no_rtdd_run]] (calls)
- [[bench/tests/test_replay.py/test_the_rtdd_strategy_builds_one_graph_per_instance_and_never_seeds]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit]] (calls)
- [[bench/tests/test_runner.py/test_run_keys_separate_the_three_modes_at_the_same_commit]] (calls)
- [[bench/tests/test_runner.py/test_a_subset_run_key_depends_on_the_selected_tests_not_their_order]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_of_the_real_suite_skips_the_second_pytest]] (calls)
- [[bench/tests/test_runner.py/test_a_parallel_run_key_is_not_the_serial_one]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_replay]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/__init__]] (method)
- [[bench/replay/cache.py/Cache/key]] (method)
- [[bench/replay/cache.py/Cache/_json_path]] (method)
- [[bench/replay/cache.py/Cache/_artifact_path]] (method)
- [[bench/replay/cache.py/Cache/_complete_marker]] (method)
- [[bench/replay/cache.py/Cache/get_json]] (method)
- [[bench/replay/cache.py/Cache/put_json]] (method)
- [[bench/replay/cache.py/Cache/json_or_build]] (method)
- [[bench/replay/cache.py/Cache/artifact]] (method)
- [[bench/replay/cache.py/Cache/store_artifact]] (method)
- [[bench/replay/cache.py/Cache/restore_artifact]] (method)
- [[bench/replay/cache.py/Cache/stats]] (method)
