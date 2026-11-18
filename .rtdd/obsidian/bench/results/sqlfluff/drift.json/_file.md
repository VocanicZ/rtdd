---
id: "bench/results/sqlfluff/drift.json"
source: "bench/results/sqlfluff/drift.json"
start: 1
end: 1
kind: "file"
is-test: false
tags: [rtdd/file]
---
# drift.json

#rtdd/file

## Tests

- [[bench/tests/test_cli.py/test_session_does_not_overwrite_the_replay_config_that_produced_the_table]] (references)
- [[bench/tests/test_report.py/test_drift_is_written_only_when_a_curve_was_measured]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_every_corpus_repo_has_every_published_file]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_every_summary_carries_a_verdict_line]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_summary_is_for_the_repo_it_sits_under]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_full_control_recalls_everything_it_could_detect]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_aggregate_spans_the_whole_corpus]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_published_summary_never_cites_a_mean_without_its_spread]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_published_docs_never_cite_a_wallclock_mean_without_its_spread]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_the_local_ci_gate_runs_the_bench_replay_suite]] (references)
- [[bench/tests/test_results_corpus_coverage.py/test_the_readme_carries_no_baseline_comparison]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_committed_records_carry_one_derived_static_record_per_commit]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_summary_carries_the_static_arm_in_every_variant]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_static_arm_publishes_no_wallclock_it_never_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_renders_the_static_wallclock_as_not_measured]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_run_config_that_stamped_the_numbers_still_never_names_the_derived_arm]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_prints_every_template_the_static_arm_models_with]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_markdown_states_the_pre_registered_static_verdict]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_published_static_verdict_agrees_with_the_committed_numbers]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_dropped_sqlfluff_result_is_byte_identical_to_what_v1_published]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_provisions_nothing_clones_nothing_and_executes_nothing]] (references)
- [[bench/tests/test_results_static_arm.py/test_the_backfill_leaves_the_ground_truth_cache_byte_identical]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[bench/replay/cli.py/cmd_session]] (references)
- [[bench/replay/report.py/write_results]] (references)
- [[bench/replay/session.py/DriftCurve]] (references)
- [[bench/results/README.md/_file]] (references)
- [[docs/plans/04-m3-replay-benchmark.md/_file]] (references)
- [[docs/plans/12-bench-rounds.md/_file]] (references)
- [[docs/results/axis2-corpus-replay.md/_file]] (references)
- [[docs/results/axis2-selection-baselines.md/_file]] (references)

## Callees

none
