---
id: "bench/replay/chart.py::head_to_head_svg"
source: "bench/replay/chart.py"
start: 633
end: 753
kind: "func"
is-test: false
tags: [rtdd/code]
---
# head_to_head_svg

#rtdd/code

## Tests

- [[bench/tests/test_chart.py/test_head_to_head_draws_only_rtdd_and_the_full_suite]] (calls)
- [[bench/tests/test_chart.py/test_head_to_head_never_plots_a_baseline_even_when_the_summary_carries_one]] (calls)
- [[bench/tests/test_chart.py/test_head_to_head_bars_are_the_committed_values_not_literals]] (calls)
- [[bench/tests/test_chart.py/test_head_to_head_carries_its_own_limits]] (calls)
- [[bench/tests/test_chart.py/test_head_to_head_refuses_a_summary_missing_an_arm]] (calls)

## Callers

- [[bench/replay/chart.py/render_figure]] (calls)

## Callees

- [[bench/replay/chart.py/ChartError]] (calls)
- [[bench/replay/chart.py/_frac]] (calls)
- [[bench/replay/chart.py/_data]] (calls)
- [[bench/replay/chart.py/_text]] (calls)
- [[bench/replay/chart.py/_colour]] (calls)
- [[bench/replay/chart.py/_svg]] (calls)
- [[bench/replay/chart.py/_source_note]] (calls)
- [[bench/replay/chart.py/_ms]] (calls)
