---
id: "bench/swebench/metrics.py::wilson_ci"
source: "bench/swebench/metrics.py"
start: 143
end: 161
kind: "func"
is-test: false
tags: [rtdd/code]
---
# wilson_ci

#rtdd/code

## Tests

- [[bench/swebench/tests/test_metrics.py/test_wilson_ci_brackets_the_point_estimate]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_wilson_ci_matches_known_values]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_wilson_ci_of_zero_successes_has_zero_lower_bound]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_wilson_ci_of_all_successes_has_a_unit_upper_bound]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_wilson_ci_of_zero_trials_is_degenerate_not_an_error]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_wilson_ci_widens_as_z_grows]] (calls)

## Callers

- [[bench/swebench/analyze.py/summarize]] (calls)
- [[bench/swebench/tests/test_analyze.py/summary]] (calls)

## Callees

none
