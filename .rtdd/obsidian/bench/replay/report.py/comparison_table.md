---
id: "bench/replay/report.py::comparison_table"
source: "bench/replay/report.py"
start: 504
end: 543
kind: "func"
is-test: false
tags: [rtdd/code]
---
# comparison_table

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_the_comparison_table_scores_static_against_rtdd_path_and_full]] (calls)
- [[bench/tests/test_report.py/test_the_comparison_table_publishes_recall_selection_ratio_and_selected_duration]] (calls)
- [[bench/tests/test_report.py/test_an_arm_with_no_wallclock_record_renders_an_explicit_not_measured_cell]] (calls)
- [[bench/tests/test_report.py/test_the_comparison_table_names_the_arm_whose_wallclock_is_not_measured]] (calls)
- [[bench/tests/test_report.py/test_the_distribution_guard_covers_the_comparison_table]] (calls)
- [[bench/tests/test_report.py/test_a_suppressed_wallclock_run_still_publishes_the_comparison_columns]] (calls)

## Callers

- [[bench/replay/report.py/render_markdown]] (calls)

## Callees

- [[bench/replay/report.py/fmt]] (calls)
- [[bench/replay/report.py/_comparison_wallclock_cells]] (calls)
