---
id: "bench/replay/metrics.py::pair_up"
source: "bench/replay/metrics.py"
start: 78
end: 96
kind: "func"
is-test: false
tags: [rtdd/code]
---
# pair_up

#rtdd/code

## Tests

- [[bench/tests/test_metrics.py/test_pair_up_refuses_mixed_repo_records]] (calls)
- [[bench/tests/test_metrics.py/test_change_and_test_level_recall_match_the_hand_computation]] (calls)
- [[bench/tests/test_metrics.py/test_green_commits_are_excluded_from_every_recall_denominator]] (calls)
- [[bench/tests/test_metrics.py/test_recall_over_an_all_green_population_is_none_not_zero]] (calls)
- [[bench/tests/test_metrics.py/test_single_killer_stratum_is_broken_out]] (calls)
- [[bench/tests/test_metrics.py/test_the_single_killer_stratum_is_reported_first]] (calls)
- [[bench/tests/test_metrics.py/test_selection_ratio_and_duration_fraction_differ_on_a_heavy_tail]] (calls)
- [[bench/tests/test_metrics.py/test_cost_metrics_include_the_green_population]] (calls)
- [[bench/tests/test_metrics.py/test_metrics_match_hand_computed_numbers_on_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/metrics.py/summarise]] (calls)

## Callees

- [[bench/replay/metrics.py/assert_single_repo]] (calls)
