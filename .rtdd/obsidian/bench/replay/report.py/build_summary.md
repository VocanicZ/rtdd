---
id: "bench/replay/report.py::build_summary"
source: "bench/replay/report.py"
start: 318
end: 347
kind: "func"
is-test: false
tags: [rtdd/code]
---
# build_summary

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_summary_has_a_row_per_strategy_and_breaks_out_the_single_killer_stratum]] (calls)
- [[bench/tests/test_report.py/test_verdict_line_states_the_pre_registered_criterion]] (calls)
- [[bench/tests/test_report.py/test_verdict_uses_the_failure_wording_verbatim_when_rtdd_costs_more_duration]] (calls)
- [[bench/tests/test_report.py/test_verdict_says_rtdd_wins_only_when_it_is_no_more_expensive]] (calls)
- [[bench/tests/test_report.py/test_verdict_refuses_to_compute_without_both_arms]] (calls)
- [[bench/tests/test_report.py/test_every_summary_markdown_carries_a_verdict_line]] (calls)
- [[bench/tests/test_report.py/test_markdown_embeds_the_config_and_the_hardware]] (calls)
- [[bench/tests/test_report.py/test_markdown_publishes_every_metric_the_prd_names]] (calls)
- [[bench/tests/test_report.py/test_the_wallclock_table_discloses_which_column_ran_in_parallel]] (calls)
- [[bench/tests/test_report.py/test_isolation_violations_survive_the_ci_wallclock_refusal]] (calls)
- [[bench/tests/test_report.py/test_wallclock_suppressed_when_operator_withholds_it]] (calls)
- [[bench/tests/test_report.py/test_variants_get_separate_tables_and_probe_is_labelled_on_the_number_line]] (calls)
- [[bench/tests/test_report.py/test_per_repo_markdown_never_names_a_second_repo]] (calls)
- [[bench/tests/test_report.py/test_aggregate_is_duration_weighted_and_says_so]] (calls)
- [[bench/tests/test_report.py/test_write_aggregate_is_the_only_cross_repo_file]] (calls)
- [[bench/tests/test_report.py/test_a_green_natural_population_still_publishes_the_probe_comparison]] (calls)
- [[bench/tests/test_report.py/test_the_probe_verdict_is_absent_when_probe_was_not_run]] (calls)
- [[bench/tests/test_report.py/test_the_markdown_carries_the_probe_verdict_beside_the_primary_one]] (calls)
- [[bench/tests/test_report.py/test_the_summary_publishes_the_cycles_where_rtdd_run_refused]] (calls)
- [[bench/tests/test_report.py/test_wallclock_rows_carry_the_distribution_beside_every_mean]] (calls)
- [[bench/tests/test_report.py/test_every_published_mean_has_a_p50_p90_and_worst_beside_it]] (calls)
- [[bench/tests/test_report.py/test_the_wallclock_markdown_table_publishes_the_distribution]] (calls)
- [[bench/tests/test_report.py/test_the_distribution_guard_accepts_the_real_rendered_report]] (calls)
- [[bench/tests/test_report.py/test_the_suppressed_wallclock_section_has_no_table_to_guard]] (calls)
- [[bench/tests/test_report.py/test_render_markdown_refuses_to_return_a_document_the_guard_rejects]] (calls)
- [[bench/tests/test_report.py/test_the_comparison_table_is_rendered_into_the_published_markdown]] (calls)
- [[bench/tests/test_report.py/test_no_comparison_section_where_the_run_carries_no_static_arm]] (calls)
- [[bench/tests/test_report.py/test_the_static_arm_section_discloses_every_template_it_models_with]] (calls)
- [[bench/tests/test_report.py/test_the_static_arm_section_names_the_level_2_source_and_both_construction_facts]] (calls)
- [[bench/tests/test_report.py/test_the_markdown_states_the_static_verdict_beside_the_comparison_table]] (calls)
- [[bench/tests/test_report.py/test_a_run_with_no_static_arm_states_no_static_verdict]] (calls)
- [[bench/tests/test_report.py/test_the_rtdd_verdict_is_never_mistaken_for_the_static_one]] (calls)

## Callers

- [[bench/replay/cli.py/_rebuild_repo]] (calls)
- [[bench/replay/report.py/write_results]] (calls)

## Callees

- [[bench/replay/falsesignal.py/summarise]] (calls)
- [[bench/replay/metrics.py/assert_single_repo]] (calls)
- [[bench/replay/metrics.py/summarise]] (calls)
- [[bench/replay/report.py/primary_variant]] (calls)
- [[bench/replay/report.py/isolation_table]] (calls)
- [[bench/replay/report.py/wallclock_table]] (calls)
- [[bench/results/flask/summary.json/_file]] (references)
- [[bench/results/httpie/summary.json/_file]] (references)
- [[bench/results/i508/flask/summary.json/_file]] (references)
- [[bench/results/n4/flask/summary.json/_file]] (references)
- [[bench/results/n4/httpie/summary.json/_file]] (references)
- [[bench/results/paired/flask/summary.json/_file]] (references)
- [[bench/results/sqlfluff/summary.json/_file]] (references)
