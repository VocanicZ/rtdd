---
id: "bench/swebench/providers/rtdd.py::render"
source: "bench/swebench/providers/rtdd.py"
start: 231
end: 270
kind: "func"
is-test: false
tags: [rtdd/code]
---
# render

#rtdd/code

## Tests

- [[bench/swebench/tests/test_report.py/test_render_emits_one_row_per_arm_in_the_pre_registered_order]] (calls)
- [[bench/swebench/tests/test_report.py/test_every_arm_row_carries_both_regression_rates_and_the_resolution_rate]] (calls)
- [[bench/swebench/tests/test_report.py/test_every_rendered_table_naming_a_regression_also_names_resolution]] (calls)
- [[bench/swebench/tests/test_report.py/test_divergence_opens_with_the_banner_carrying_the_note]] (calls)
- [[bench/swebench/tests/test_report.py/test_divergence_strips_the_published_column_from_header_separator_and_rows]] (calls)
- [[bench/swebench/tests/test_report.py/test_reproducing_output_carries_no_banner]] (calls)
- [[bench/swebench/tests/test_report.py/test_the_published_column_is_populated_only_for_the_three_published_arms]] (calls)
- [[bench/swebench/tests/test_report.py/test_rates_render_at_a_fixed_precision_so_tables_diff_cleanly]] (calls)
- [[bench/swebench/tests/test_report.py/test_rendering_the_same_summary_twice_is_byte_identical]] (calls)
- [[bench/swebench/tests/test_report.py/test_pairwise_rows_carry_both_resolution_rates_beside_the_regression_delta]] (calls)
- [[bench/swebench/tests/test_report.py/test_the_kill_criterion_verdict_is_printed_with_its_signed_floor]] (calls)

## Callers

- [[bench/swebench/providers/rtdd.py/which_tool/call]] (calls)

## Callees

- [[bench/swebench/providers/rtdd.py/_strip_remedy]] (calls)
