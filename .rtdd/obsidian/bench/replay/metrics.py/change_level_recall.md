---
id: "bench/replay/metrics.py::change_level_recall"
source: "bench/replay/metrics.py"
start: 104
end: 107
kind: "func"
is-test: false
tags: [rtdd/code]
---
# change_level_recall

#rtdd/code

## Tests

- [[bench/tests/test_metrics.py/test_change_and_test_level_recall_match_the_hand_computation]] (calls)
- [[bench/tests/test_metrics.py/test_green_commits_are_excluded_from_every_recall_denominator]] (calls)
- [[bench/tests/test_metrics.py/test_recall_over_an_all_green_population_is_none_not_zero]] (calls)
- [[bench/tests/test_metrics.py/test_single_killer_stratum_is_broken_out]] (calls)
- [[bench/tests/test_metrics.py/test_metrics_match_hand_computed_numbers_on_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/metrics.py/summarise]] (calls)

## Callees

- [[bench/replay/metrics.py/Ratio]] (calls)
- [[bench/replay/metrics.py/detected]] (calls)
- [[bench/replay/metrics.py/_detecting]] (calls)
