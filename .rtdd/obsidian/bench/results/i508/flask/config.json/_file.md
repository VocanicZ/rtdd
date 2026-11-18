---
id: "bench/results/i508/flask/config.json"
source: "bench/results/i508/flask/config.json"
start: 1
end: 1
kind: "file"
is-test: false
tags: [rtdd/file]
---
# config.json

#rtdd/file

## Tests

- [[bench/swebench/tests/test_providers_tdad.py/test_the_pin_is_recorded_in_the_published_config]] (references)
- [[bench/swebench/tests/test_report.py/test_write_config_writes_into_the_swebench_results_directory]] (references)
- [[bench/tests/test_audit.py/test_published_results_all_stamp_a_digest_the_lock_still_knows]] (references)
- [[bench/tests/test_cli.py/test_session_does_not_overwrite_the_replay_config_that_produced_the_table]] (references)
- [[bench/tests/test_cli.py/test_rebuild_leaves_the_run_config_that_stamped_the_result_untouched]] (references)
- [[bench/tests/test_cli.py/test_derive_leaves_config_json_untouched]] (references)
- [[bench/tests/test_cli.py/test_rebuild_publishes_an_arm_the_records_carry_and_the_config_does_not]] (references)
- [[bench/tests/test_report.py/test_write_results_is_byte_stable_across_reruns]] (references)
- [[bench/tests/test_report.py/test_config_json_carries_everything_needed_to_reproduce_the_numbers]] (references)
- [[bench/tests/test_report.py/test_no_result_file_is_written_without_its_config]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_every_corpus_repo_has_every_published_file]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_every_summary_carries_a_verdict_line]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_summary_is_for_the_repo_it_sits_under]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_full_control_recalls_everything_it_could_detect]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_aggregate_spans_the_whole_corpus]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_published_summary_never_cites_a_mean_without_its_spread]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_published_docs_never_cite_a_wallclock_mean_without_its_spread]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_the_local_ci_gate_runs_the_bench_replay_suite]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_the_readme_carries_no_baseline_comparison]] (references)
- [[bench/tests/test_results_i508_resolution.py/test_the_page_states_both_binaries_and_hardware]] (references)
- [[bench/tests/test_results_n4_rounds.py/test_every_corpus_row_was_replayed_with_both_arms_and_tdd]] (references)
- [[bench/tests/test_results_n4_rounds.py/test_the_page_states_the_binary_corpus_and_hardware]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_committed_records_carry_one_derived_static_record_per_commit]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_summary_carries_the_static_arm_in_every_variant]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_static_arm_publishes_no_wallclock_it_never_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_renders_the_static_wallclock_as_not_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_run_config_that_stamped_the_numbers_still_never_names_the_derived_arm]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_run_config_that_stamped_the_numbers_still_never_names_the_derived_arm]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_prints_every_template_the_static_arm_models_with]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_states_the_pre_registered_static_verdict]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_static_verdict_agrees_with_the_committed_numbers]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_dropped_sqlfluff_result_is_byte_identical_to_what_v1_published]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_provisions_nothing_clones_nothing_and_executes_nothing]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_leaves_the_ground_truth_cache_byte_identical]] (references)
- [[internal/paths/glob_test.go/TestMatchGlob]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[bench/replay/cli.py/cmd_session]] (references)
- [[bench/replay/cli.py/_rebuild_repo]] (references)
- [[bench/replay/cli.py/cmd_rebuild]] (references)
- [[bench/replay/cli.py/build_parser]] (references)
- [[bench/replay/report.py/write_results]] (references)
- [[bench/results/README.md/_file]] (references)
- [[bench/swebench/report.py/write_config]] (references)
- [[bench/tests/test_cli.py/_published_repo]] (references)
- [[docs/plans/01-m1a-core.md/_file]] (references)
- [[docs/plans/04-m3-replay-benchmark.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/06-m6e-evidence-and-honesty.md/_file]] (references)
- [[docs/plans/12-bench-rounds.md/_file]] (references)
- [[docs/results/axis2-selection-baselines.md/_file]] (references)
- [[docs/results/n4-bench-rounds.md/_file]] (references)

## Callees

none
