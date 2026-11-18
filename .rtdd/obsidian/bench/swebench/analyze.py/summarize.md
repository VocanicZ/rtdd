---
id: "bench/swebench/analyze.py::summarize"
source: "bench/swebench/analyze.py"
start: 96
end: 117
kind: "func"
is-test: false
tags: [rtdd/code]
---
# summarize

#rtdd/code

## Tests

- [[bench/swebench/tests/test_analyze.py/test_summarize_reports_resolution_beside_every_regression_rate]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_summarize_counts_unapplied_patches_and_harness_errors_separately]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_an_arm_that_produced_nothing_scores_zero_regressions_and_zero_resolution]] (calls)

## Callers

- [[bench/swebench/analyze.py/analyze]] (calls)

## Callees

- [[bench/swebench/analyze.py/ArmSummary]] (calls)
- [[bench/swebench/metrics.py/test_level_regression_rate]] (calls)
- [[bench/swebench/metrics.py/instance_level_regression_rate]] (calls)
- [[bench/swebench/metrics.py/resolution_rate]] (calls)
- [[bench/swebench/metrics.py/regression_counts]] (calls)
- [[bench/swebench/metrics.py/wilson_ci]] (calls)
