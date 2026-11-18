---
id: "bench/replay/report.py::assert_distribution_beside_mean"
source: "bench/replay/report.py"
start: 147
end: 186
kind: "func"
is-test: false
tags: [rtdd/code]
---
# assert_distribution_beside_mean

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_a_wallclock_table_that_cites_only_a_mean_is_refused]] (calls)
- [[bench/tests/test_report.py/test_a_wallclock_table_with_no_mean_at_all_is_still_refused]] (calls)
- [[bench/tests/test_report.py/test_a_mean_outside_the_wallclock_section_needs_its_distribution_too]] (calls)
- [[bench/tests/test_report.py/test_the_distribution_guard_accepts_the_real_rendered_report]] (calls)
- [[bench/tests/test_report.py/test_the_suppressed_wallclock_section_has_no_table_to_guard]] (calls)
- [[bench/tests/test_report.py/test_the_distribution_guard_covers_the_comparison_table]] (calls)
- [[bench/tests/test_report.py/test_a_comparison_table_carrying_a_bare_mean_is_refused]] (calls)
- [[bench/tests/test_report.py/test_the_guard_is_not_satisfiable_by_omitting_the_wallclock_column]] (calls)
- [[bench/tests/test_report.py/test_a_suppressed_wallclock_run_still_publishes_the_comparison_columns]] (calls)
- [[bench/tests/test_report.py/test_the_comparison_table_is_rendered_into_the_published_markdown]] (calls)
- [[bench/tests/test_results_corpus_coverage.py/test_published_summary_never_cites_a_mean_without_its_spread]] (calls)
- [[bench/tests/test_results_corpus_coverage.py/test_published_docs_never_cite_a_wallclock_mean_without_its_spread]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_the_rounds_table_puts_tdd_beside_both_arms]] (calls)

## Callers

- [[bench/replay/report.py/render_markdown]] (calls)

## Callees

- [[bench/replay/report.py/ReportError]] (calls)
- [[bench/replay/report.py/_tables]] (calls)
- [[bench/replay/report.py/_is_wallclock_header]] (calls)
- [[bench/replay/report.py/_is_comparison_header]] (calls)
