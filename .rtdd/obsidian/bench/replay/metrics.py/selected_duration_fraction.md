---
id: "bench/replay/metrics.py::selected_duration_fraction"
source: "bench/replay/metrics.py"
start: 145
end: 149
kind: "func"
is-test: false
tags: [rtdd/code]
---
# selected_duration_fraction

#rtdd/code

## Tests

- [[bench/tests/test_metrics.py/test_selection_ratio_and_duration_fraction_differ_on_a_heavy_tail]] (calls)
- [[bench/tests/test_metrics.py/test_cost_metrics_include_the_green_population]] (calls)

## Callers

- [[bench/replay/metrics.py/summarise]] (calls)

## Callees

- [[bench/replay/metrics.py/Ratio]] (calls)
- [[bench/replay/records.py/CommitRecord/total_duration_ms]] (calls)
