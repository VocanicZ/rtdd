---
id: "bench/replay/chart.py::safety_svg"
source: "bench/replay/chart.py"
start: 309
end: 445
kind: "func"
is-test: false
tags: [rtdd/code]
---
# safety_svg

#rtdd/code

## Tests

- [[bench/tests/test_chart.py/test_safety_never_plots_a_null_recall_as_zero]] (calls)
- [[bench/tests/test_chart.py/test_safety_names_the_strategies_it_could_not_plot]] (calls)
- [[bench/tests/test_chart.py/test_safety_plots_the_population_that_does_have_ground_truth]] (calls)
- [[bench/tests/test_chart.py/test_a_real_zero_recall_is_plotted_and_a_null_one_is_not]] (calls)
- [[bench/tests/test_chart.py/test_safety_marks_the_upper_bound_when_the_variant_is_probe]] (calls)
- [[bench/tests/test_chart.py/test_safety_figure_carries_the_pre_registered_verdict]] (calls)
- [[bench/tests/test_chart.py/test_safety_verdict_is_derived_not_typed]] (calls)

## Callers

- [[bench/replay/chart.py/render_figure]] (calls)

## Callees

- [[bench/replay/chart.py/_frac]] (calls)
- [[bench/replay/chart.py/_data]] (calls)
- [[bench/replay/chart.py/_text]] (calls)
- [[bench/replay/chart.py/_colour]] (calls)
- [[bench/replay/chart.py/_wrap]] (calls)
- [[bench/replay/chart.py/_svg]] (calls)
- [[bench/replay/chart.py/_source_note]] (calls)
- [[bench/replay/chart.py/_placed]] (calls)
- [[bench/replay/chart.py/_verdict_for]] (calls)
- [[bench/replay/report.py/_file]] (references)
- [[bench/results/flask/summary.json/_file]] (references)
- [[bench/results/httpie/summary.json/_file]] (references)
- [[bench/results/i508/flask/summary.json/_file]] (references)
- [[bench/results/n4/flask/summary.json/_file]] (references)
- [[bench/results/n4/httpie/summary.json/_file]] (references)
- [[bench/results/paired/flask/summary.json/_file]] (references)
- [[bench/results/sqlfluff/summary.json/_file]] (references)
- [[bench/swebench/report.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/edit/src/report.py/_file]] (references)
- [[cmd/rtdd/testdata/which-golden/repo/src/report.py/_file]] (references)
