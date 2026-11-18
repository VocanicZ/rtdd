---
id: "bench/replay/metrics.py::Ratio::value"
source: "bench/replay/metrics.py"
start: 51
end: 52
kind: "method"
is-test: false
tags: [rtdd/code]
---
# value

#rtdd/code

## Tests

- [[bench/tests/test_falsesignal.py/test_silent_commits_are_excluded_from_the_rate_but_counted_in_fire_rate]] (calls)
- [[bench/tests/test_falsesignal.py/test_a_rate_with_no_fired_commit_has_no_value_rather_than_a_zero]] (calls)
- [[bench/tests/test_falsesignal.py/test_false_signal_rate_against_the_synthetic_repo]] (calls)
- [[bench/tests/test_falsesignal.py/test_truth_is_only_the_instrumented_run_never_the_reported_set]] (calls)
- [[bench/tests/test_metrics.py/test_change_and_test_level_recall_match_the_hand_computation]] (calls)
- [[bench/tests/test_metrics.py/test_recall_over_an_all_green_population_is_none_not_zero]] (calls)
- [[bench/tests/test_metrics.py/test_single_killer_stratum_is_broken_out]] (calls)
- [[bench/tests/test_metrics.py/test_selection_ratio_and_duration_fraction_differ_on_a_heavy_tail]] (calls)
- [[bench/tests/test_metrics.py/test_escalation_rate_counts_every_cycle]] (calls)

## Callers

- [[bench/replay/metrics.py/Ratio]] (method)
- [[bench/replay/metrics.py/Ratio/to_dict]] (calls)

## Callees

none
