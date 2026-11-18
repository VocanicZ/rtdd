---
id: "bench/swebench/metrics.py::instance_level_regression_rate"
source: "bench/swebench/metrics.py"
start: 115
end: 119
kind: "func"
is-test: false
tags: [rtdd/code]
---
# instance_level_regression_rate

#rtdd/code

## Tests

- [[bench/swebench/tests/test_metrics.py/test_regressions_are_read_from_nothing_but_the_p2p_failure_list]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_unapplied_patch_has_no_regressions_but_stays_in_the_denominator]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_test_level_rate_pools_tests_not_instances]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_instance_level_rate_counts_instances_with_at_least_one_regression]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_empty_sample_rates_are_zero]] (calls)

## Callers

- [[bench/swebench/analyze.py/summarize]] (calls)

## Callees

none
