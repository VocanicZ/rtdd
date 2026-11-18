---
id: "bench/replay/chart.py::savings_svg"
source: "bench/replay/chart.py"
start: 210
end: 303
kind: "func"
is-test: false
tags: [rtdd/code]
---
# savings_svg

#rtdd/code

## Tests

- [[bench/tests/test_chart.py/test_savings_plots_every_strategy_the_summary_carries]] (calls)
- [[bench/tests/test_chart.py/test_savings_publishes_both_the_test_count_and_the_duration]] (calls)
- [[bench/tests/test_chart.py/test_savings_bar_length_is_the_committed_fraction_not_a_literal]] (calls)
- [[bench/tests/test_chart.py/test_the_full_suite_is_drawn_at_the_full_width]] (calls)
- [[bench/tests/test_chart.py/test_dark_palette_actually_changes_the_ink]] (calls)

## Callers

- [[bench/replay/chart.py/render_figure]] (calls)

## Callees

- [[bench/replay/chart.py/_frac]] (calls)
- [[bench/replay/chart.py/_data]] (calls)
- [[bench/replay/chart.py/_text]] (calls)
- [[bench/replay/chart.py/_colour]] (calls)
- [[bench/replay/chart.py/_svg]] (calls)
- [[bench/replay/chart.py/_source_note]] (calls)
- [[bench/replay/chart.py/_ordered]] (calls)
- [[bench/results/flask/summary.json/_file]] (references)
- [[bench/results/httpie/summary.json/_file]] (references)
- [[bench/results/i508/flask/summary.json/_file]] (references)
- [[bench/results/n4/flask/summary.json/_file]] (references)
- [[bench/results/n4/httpie/summary.json/_file]] (references)
- [[bench/results/paired/flask/summary.json/_file]] (references)
- [[bench/results/sqlfluff/summary.json/_file]] (references)
