---
id: "bench/replay/cli.py::main"
source: "bench/replay/cli.py"
start: 663
end: 669
kind: "func"
is-test: false
tags: [rtdd/code]
---
# main

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_replay_refuses_a_repo_outside_the_frozen_corpus]] (calls)
- [[bench/tests/test_cli.py/test_session_refuses_a_repo_outside_the_frozen_corpus]] (calls)
- [[bench/tests/test_cli.py/test_replay_refuses_an_unfrozen_corpus_before_doing_any_work]] (calls)
- [[bench/tests/test_cli.py/test_replay_refuses_random_without_its_peer_before_any_clone]] (calls)
- [[bench/tests/test_cli.py/test_replay_allows_random_when_its_peer_is_in_the_same_run]] (calls)
- [[bench/tests/test_cli.py/test_replay_refuses_rounds_1_2_without_the_round_1_arm_before_any_clone]] (calls)
- [[bench/tests/test_cli.py/test_replay_allows_rounds_1_2_when_rtdd_is_in_the_same_run]] (calls)
- [[bench/tests/test_cli.py/test_replay_writes_a_labelled_run_beside_the_published_result]] (calls)
- [[bench/tests/test_cli.py/test_replay_refuses_a_label_that_is_not_one_fresh_path_segment_before_any_clone]] (calls)
- [[bench/tests/test_cli.py/test_replay_refuses_wall_clock_on_ci_and_still_computes_the_rest]] (calls)
- [[bench/tests/test_cli.py/test_replay_measures_wall_clock_off_ci]] (calls)
- [[bench/tests/test_cli.py/test_replay_honours_no_wallclock_without_claiming_ci]] (calls)
- [[bench/tests/test_cli.py/test_doctor_reports_ci_and_the_corpus_digest]] (calls)
- [[bench/tests/test_cli.py/test_doctor_lists_all_seven_baselines_plus_rtdd]] (calls)
- [[bench/tests/test_cli.py/test_doctor_reports_the_resolved_config_and_the_tool_versions]] (calls)
- [[bench/tests/test_cli.py/test_doctor_exits_non_zero_when_the_rtdd_binary_is_absent]] (calls)
- [[bench/tests/test_cli.py/test_doctor_exits_non_zero_when_the_corpus_is_not_frozen]] (calls)
- [[bench/tests/test_cli.py/test_report_refuses_when_no_per_repo_summary_has_been_committed]] (calls)
- [[bench/tests/test_cli.py/test_report_regenerates_the_aggregate_from_committed_summaries]] (calls)
- [[bench/tests/test_cli.py/test_report_keeps_but_does_not_weigh_a_result_the_corpus_no_longer_admits]] (calls)
- [[bench/tests/test_cli.py/test_audit_passes_on_a_corpus_that_meets_its_own_criteria]] (calls)
- [[bench/tests/test_cli.py/test_audit_fails_and_names_the_repo_that_breaches]] (calls)
- [[bench/tests/test_cli.py/test_replay_runs_the_corpus_repo_in_its_own_provisioned_interpreter]] (calls)
- [[bench/tests/test_cli.py/test_replay_commits_can_be_bounded_and_the_bound_is_published]] (calls)
- [[bench/tests/test_cli.py/test_replay_defaults_to_the_frozen_commit_count]] (calls)
- [[bench/tests/test_cli.py/test_session_runs_the_drift_worktree_with_its_own_source_in_front]] (calls)
- [[bench/tests/test_cli.py/test_session_does_not_overwrite_the_replay_config_that_produced_the_table]] (calls)
- [[bench/tests/test_cli.py/test_session_threads_corpus_version_through_to_the_loader]] (calls)
- [[bench/tests/test_cli.py/test_report_threads_corpus_version_through_to_the_loader]] (calls)
- [[bench/tests/test_cli.py/test_report_rebuilds_a_summary_from_the_committed_records]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_leaves_the_run_config_that_stamped_the_result_untouched]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_does_not_touch_a_repo_the_corpus_no_longer_admits]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_refuses_a_results_directory_with_no_records]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_keeps_an_operator_wallclock_refusal_refused]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_preserves_what_the_records_do_not_carry]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_publishes_strategies_in_the_order_the_run_did]] (calls)
- [[bench/tests/test_cli.py/test_derive_appends_a_static_record_per_commit]] (calls)
- [[bench/tests/test_cli.py/test_derive_is_idempotent_to_the_byte]] (calls)
- [[bench/tests/test_cli.py/test_derive_supersedes_a_stale_derived_record_rather_than_doubling_it]] (calls)
- [[bench/tests/test_cli.py/test_derive_leaves_the_measured_records_exactly_as_the_run_wrote_them]] (calls)
- [[bench/tests/test_cli.py/test_derive_never_touches_a_repo_the_corpus_no_longer_admits]] (calls)
- [[bench/tests/test_cli.py/test_derive_refuses_a_repo_whose_records_cannot_support_it]] (calls)
- [[bench/tests/test_cli.py/test_derive_leaves_config_json_untouched]] (calls)
- [[bench/tests/test_cli.py/test_derive_refuses_when_there_is_nothing_to_derive]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_publishes_an_arm_the_records_carry_and_the_config_does_not]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_orders_a_derived_arm_the_way_strategy_order_does]] (calls)
- [[bench/tests/test_cli.py/test_the_rebuilt_markdown_publishes_the_derived_arm_as_not_measured]] (calls)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_provisions_nothing_clones_nothing_and_executes_nothing]] (calls)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_leaves_the_ground_truth_cache_byte_identical]] (calls)

## Callers

none

## Callees

- [[bench/replay/cli.py/build_parser]] (calls)
