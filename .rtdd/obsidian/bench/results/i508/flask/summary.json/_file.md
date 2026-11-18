---
id: "bench/results/i508/flask/summary.json"
source: "bench/results/i508/flask/summary.json"
start: 1
end: 1
kind: "file"
is-test: false
tags: [rtdd/file]
---
# summary.json

#rtdd/file

## Tests

- [[bench/swebench/tests/test_acceptance.py/test_the_pipeline_runs_end_to_end_from_raw_records_to_a_rendered_table]] (references)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_ignores_axis_1s_own_results_directory]] (references)
- [[bench/swebench/tests/test_analyze.py/test_analyze_writes_the_summary_file_with_every_arm]] (references)
- [[bench/tests/test_cli.py/test_report_regenerates_the_aggregate_from_committed_summaries]] (references)
- [[bench/tests/test_cli.py/test_report_keeps_but_does_not_weigh_a_result_the_corpus_no_longer_admits]] (references)
- [[bench/tests/test_cli.py/test_report_rebuilds_a_summary_from_the_committed_records]] (references)
- [[bench/tests/test_cli.py/test_rebuild_does_not_touch_a_repo_the_corpus_no_longer_admits]] (references)
- [[bench/tests/test_cli.py/test_rebuild_keeps_an_operator_wallclock_refusal_refused]] (references)
- [[bench/tests/test_cli.py/test_rebuild_preserves_what_the_records_do_not_carry]] (references)
- [[bench/tests/test_cli.py/test_rebuild_publishes_an_arm_the_records_carry_and_the_config_does_not]] (references)
- [[bench/tests/test_report.py/test_write_results_is_byte_stable_across_reruns]] (references)
- [[bench/tests/test_report.py/test_summary_json_has_sorted_keys_and_a_trailing_newline]] (references)
- [[bench/tests/test_report.py/test_no_result_file_is_written_without_its_config]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_every_corpus_repo_has_every_published_file]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_every_summary_carries_a_verdict_line]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_summary_is_for_the_repo_it_sits_under]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_summary_is_for_the_repo_it_sits_under]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_full_control_recalls_everything_it_could_detect]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_full_control_recalls_everything_it_could_detect]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_aggregate_spans_the_whole_corpus]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_published_summary_never_cites_a_mean_without_its_spread]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_published_docs_never_cite_a_wallclock_mean_without_its_spread]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_the_local_ci_gate_runs_the_bench_replay_suite]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_the_readme_carries_no_baseline_comparison]] (references)
- [[bench/tests/test_results_i508_resolution.py/test_each_row_quotes_its_summary]] (references)
- [[bench/tests/test_results_n4_rounds.py/test_every_corpus_row_was_replayed_with_both_arms_and_tdd]] (references)
- [[bench/tests/test_results_n4_rounds.py/test_the_page_quotes_each_row_as_its_summary_has_it]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_committed_records_carry_one_derived_static_record_per_commit]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_summary_carries_the_static_arm_in_every_variant]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_summary_carries_the_static_arm_in_every_variant]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_static_arm_publishes_no_wallclock_it_never_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_static_arm_publishes_no_wallclock_it_never_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_renders_the_static_wallclock_as_not_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_run_config_that_stamped_the_numbers_still_never_names_the_derived_arm]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_prints_every_template_the_static_arm_models_with]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_states_the_pre_registered_static_verdict]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_static_verdict_agrees_with_the_committed_numbers]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_static_verdict_agrees_with_the_committed_numbers]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_dropped_sqlfluff_result_is_byte_identical_to_what_v1_published]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_provisions_nothing_clones_nothing_and_executes_nothing]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_leaves_the_ground_truth_cache_byte_identical]] (references)
- [[internal/installtest/outcomes_test.go/TestREADMESafetyAxisMatchesThePairedRun]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[bench/replay/chart.py/_file]] (references)
- [[bench/replay/chart.py/savings_svg]] (references)
- [[bench/replay/chart.py/safety_svg]] (references)
- [[bench/replay/chart.py/wallclock_svg]] (references)
- [[bench/replay/cli.py/_rebuild_repo]] (references)
- [[bench/replay/cli.py/cmd_report]] (references)
- [[bench/replay/cli.py/cmd_chart]] (references)
- [[bench/replay/report.py/build_summary]] (references)
- [[bench/replay/report.py/write_results]] (references)
- [[bench/results/README.md/_file]] (references)
- [[bench/swebench/analyze.py/_axis2_stratum_recalls]] (references)
- [[bench/swebench/analyze.py/analyze]] (references)
- [[bench/swebench/report.py/render]] (references)
- [[bench/swebench/report.py/main]] (references)
- [[bench/swebench/tests/test_acceptance.py/write_m3_recall]] (references)
- [[bench/swebench/tests/test_analyze.py/write_m3]] (references)
- [[bench/tests/test_chart.py/flask]] (references)
- [[bench/tests/test_chart.py/httpie]] (references)
- [[bench/tests/test_chart.py/paired]] (references)
- [[bench/tests/test_report.py/_synthetic_summary]] (references)
- [[bench/tests/test_results_n4_rounds.py/_summary]] (references)
- [[docs/plans/04-m3-replay-benchmark.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/06-m6e-evidence-and-honesty.md/_file]] (references)
- [[docs/plans/12-bench-rounds.md/_file]] (references)
- [[docs/results/axis2-first-replay.md/_file]] (references)
- [[docs/results/figures/axis2-savings-dark.svg/_file]] (references)
- [[docs/results/figures/axis2-savings-light.svg/_file]] (references)
- [[docs/results/figures/axis2-wallclock-dark.svg/_file]] (references)
- [[docs/results/figures/axis2-wallclock-light.svg/_file]] (references)
- [[docs/results/i508-scoped-resolution.md/_file]] (references)
- [[docs/results/n4-bench-rounds.md/_file]] (references)

## Callees

none
