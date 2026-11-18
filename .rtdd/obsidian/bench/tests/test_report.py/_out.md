---
id: "bench/tests/test_report.py::_out"
source: "bench/tests/test_report.py"
start: 61
end: 86
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _out

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_summary_has_a_row_per_strategy_and_breaks_out_the_single_killer_stratum]] (calls)
- [[bench/tests/test_report.py/test_verdict_line_states_the_pre_registered_criterion]] (calls)
- [[bench/tests/test_report.py/test_verdict_uses_the_failure_wording_verbatim_when_rtdd_costs_more_duration]] (calls)
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
- [[bench/tests/test_report.py/test_write_results_is_byte_stable_across_reruns]] (calls)
- [[bench/tests/test_report.py/test_commits_jsonl_is_one_sorted_json_line_per_record]] (calls)
- [[bench/tests/test_report.py/test_summary_json_has_sorted_keys_and_a_trailing_newline]] (calls)
- [[bench/tests/test_report.py/test_config_json_carries_everything_needed_to_reproduce_the_numbers]] (calls)
- [[bench/tests/test_report.py/test_no_result_file_is_written_without_its_config]] (calls)
- [[bench/tests/test_report.py/test_drift_is_written_only_when_a_curve_was_measured]] (calls)
- [[bench/tests/test_report.py/test_write_aggregate_is_the_only_cross_repo_file]] (calls)
- [[bench/tests/test_report.py/test_the_probe_verdict_is_absent_when_probe_was_not_run]] (calls)
- [[bench/tests/test_report.py/test_the_summary_publishes_the_cycles_where_rtdd_run_refused]] (calls)
- [[bench/tests/test_report.py/test_no_comparison_section_where_the_run_carries_no_static_arm]] (calls)
- [[bench/tests/test_report.py/test_a_run_with_no_static_arm_states_no_static_verdict]] (calls)

## Callers

- [[bench/tests/test_report.py/_spread_out]] (calls)
- [[bench/tests/test_report.py/_static_out]] (calls)

## Callees

- [[bench/replay/hardware.py/Hardware/fingerprint]] (calls)
- [[bench/replay/records.py/CommitRecord]] (calls)
- [[bench/replay/records.py/StrategyRecord]] (calls)
- [[bench/replay/records.py/WallClockRecord]] (calls)
- [[bench/replay/records.py/UncoveredRecord]] (calls)
- [[bench/replay/replay.py/ReplayOutput]] (calls)
