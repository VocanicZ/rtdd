---
id: "bench/swebench/metrics.py::resolution_rate"
source: "bench/swebench/metrics.py"
start: 122
end: 135
kind: "func"
is-test: false
tags: [rtdd/code]
---
# resolution_rate

#rtdd/code

## Tests

- [[bench/swebench/tests/test_metrics.py/test_unapplied_patch_has_no_regressions_but_stays_in_the_denominator]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_a_crashed_evaluation_is_unevaluated_but_keeps_its_denominator]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_a_do_nothing_arm_cannot_win]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_resolution_requires_all_f2p_and_no_p2p_failure]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_resolution_rate_denominator_is_the_whole_sample]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_empty_sample_rates_are_zero]] (calls)

## Callers

- [[bench/swebench/analyze.py/summarize]] (calls)

## Callees

none
