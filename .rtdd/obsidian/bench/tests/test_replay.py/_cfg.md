---
id: "bench/tests/test_replay.py::_cfg"
source: "bench/tests/test_replay.py"
start: 44
end: 59
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _cfg

#rtdd/code

## Tests

- [[bench/tests/test_replay.py/test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth]] (calls)
- [[bench/tests/test_replay.py/test_f_full_subtracts_the_failures_already_red_at_the_clean_parent]] (calls)
- [[bench/tests/test_replay.py/test_the_two_populations_are_tagged_and_never_pooled]] (calls)
- [[bench/tests/test_replay.py/test_every_cycle_produces_a_strategy_record_for_every_strategy]] (calls)
- [[bench/tests/test_replay.py/test_a_commit_that_collects_nothing_is_skipped_and_the_replay_continues]] (calls)
- [[bench/tests/test_replay.py/test_wall_clock_captures_all_three_forms_and_the_isolation_check]] (calls)
- [[bench/tests/test_replay.py/test_wall_clock_is_suppressed_on_ci]] (calls)
- [[bench/tests/test_replay.py/test_a_cached_run_replayed_under_different_hardware_does_not_publish_borrowed_timing]] (calls)
- [[bench/tests/test_replay.py/test_a_repo_outside_the_frozen_corpus_is_refused]] (calls)
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

## Callers

none

## Callees

- [[bench/replay/config.py/RunConfig]] (calls)
