---
id: "bench/swebench/metrics.py::from_report"
source: "bench/swebench/metrics.py"
start: 62
end: 104
kind: "func"
is-test: false
tags: [rtdd/code]
---
# from_report

#rtdd/code

## Tests

- [[bench/swebench/tests/test_metrics.py/test_regression_is_a_p2p_failure]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_regressions_are_read_from_nothing_but_the_p2p_failure_list]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_denominator_comes_from_the_dataset_not_the_report]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_dataset_row_may_carry_plain_lists_as_well_as_json_strings]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_unapplied_patch_has_no_regressions_but_stays_in_the_denominator]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_a_crashed_evaluation_is_unevaluated_but_keeps_its_denominator]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_a_do_nothing_arm_cannot_win]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_test_level_rate_pools_tests_not_instances]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_instance_level_rate_counts_instances_with_at_least_one_regression]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_resolution_requires_all_f2p_and_no_p2p_failure]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_resolution_rate_denominator_is_the_whole_sample]] (calls)
- [[bench/swebench/tests/test_metrics.py/test_instance_result_is_frozen]] (calls)

## Callers

- [[bench/swebench/evaluate.py/collect]] (calls)

## Callees

- [[bench/swebench/metrics.py/InstanceResult]] (calls)
- [[bench/swebench/metrics.py/_ids]] (calls)
