---
id: "bench/replay/chart.py::wallclock_svg"
source: "bench/replay/chart.py"
start: 494
end: 606
kind: "func"
is-test: false
tags: [rtdd/code]
---
# wallclock_svg

#rtdd/code

## Tests

- [[bench/tests/test_chart.py/test_wallclock_publishes_the_distribution_never_a_bare_mean]] (calls)
- [[bench/tests/test_chart.py/test_wallclock_draws_the_full_suite_reference]] (calls)
- [[bench/tests/test_chart.py/test_wallclock_omits_a_strategy_with_no_recorded_run]] (calls)

## Callers

- [[bench/replay/chart.py/render_figure]] (calls)

## Callees

- [[bench/replay/chart.py/_data]] (calls)
- [[bench/replay/chart.py/_text]] (calls)
- [[bench/replay/chart.py/_colour]] (calls)
- [[bench/replay/chart.py/_svg]] (calls)
- [[bench/replay/chart.py/_source_note]] (calls)
- [[bench/replay/chart.py/wallclock_svg/px]] (calls)
- [[bench/results/flask/summary.json/_file]] (references)
- [[bench/results/httpie/summary.json/_file]] (references)
- [[bench/results/i508/flask/summary.json/_file]] (references)
- [[bench/results/n4/flask/summary.json/_file]] (references)
- [[bench/results/n4/httpie/summary.json/_file]] (references)
- [[bench/results/paired/flask/summary.json/_file]] (references)
- [[bench/results/sqlfluff/summary.json/_file]] (references)
