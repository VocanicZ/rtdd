---
id: "bench/replay/replay.py::replay_repo"
source: "bench/replay/replay.py"
start: 295
end: 563
kind: "func"
is-test: false
tags: [rtdd/code]
---
# replay_repo

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

## Callers

- [[bench/replay/cli.py/cmd_replay]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/key]] (calls)
- [[bench/replay/corpus.py/Corpus/require]] (calls)
- [[bench/replay/envsetup.py/with_source_path]] (calls)
- [[bench/replay/gitwork.py/replay_points]] (calls)
- [[bench/replay/gitwork.py/paired_points]] (calls)
- [[bench/replay/gitwork.py/add_worktree]] (calls)
- [[bench/replay/gitwork.py/remove_worktree]] (calls)
- [[bench/replay/gitwork.py/materialise_natural]] (calls)
- [[bench/replay/gitwork.py/materialise_probe]] (calls)
- [[bench/replay/hardware.py/Hardware/fingerprint]] (calls)
- [[bench/replay/hardware.py/Hardware/wallclock_allowed]] (calls)
- [[bench/replay/records.py/CommitRecord]] (calls)
- [[bench/replay/records.py/StrategyRecord]] (calls)
- [[bench/replay/records.py/WallClockRecord]] (calls)
- [[bench/replay/replay.py/_material_changes]] (calls)
- [[bench/replay/replay.py/ReplayOutput]] (calls)
- [[bench/replay/replay.py/strategy_order]] (calls)
- [[bench/replay/replay.py/isolation_violation]] (calls)
- [[bench/replay/replay.py/_prepare_state]] (calls)
- [[bench/replay/replay.py/_cached_full]] (calls)
- [[bench/replay/replay.py/clean_tree_failures]] (calls)
- [[bench/replay/replay.py/_cached_selection]] (calls)
- [[bench/replay/replay.py/_cached_collect]] (calls)
- [[bench/replay/runner.py/RunResult/failing]] (calls)
- [[bench/replay/runner.py/RunResult/durations]] (calls)
- [[bench/replay/runner.py/run_subset]] (calls)
- [[bench/replay/runner.py/run_key]] (calls)
- [[bench/replay/runner.py/cached_run]] (calls)
- [[bench/replay/strategies/base.py/CommitContext]] (calls)
- [[bench/replay/strategies/base.py/register]] (calls)
- [[bench/replay/strategies/base.py/get]] (calls)
- [[bench/replay/strategies/base.py/validate_selection]] (calls)
- [[bench/replay/strategies/randomratio.py/RandomRatio]] (calls)
- [[bench/replay/strategies/rtdd.py/Rtdd]] (calls)
- [[bench/replay/strategies/rtdd.py/RtddRounds12]] (calls)
