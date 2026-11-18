---
id: "bench/replay/metrics.py::summarise"
source: "bench/replay/metrics.py"
start: 176
end: 205
kind: "func"
is-test: false
tags: [rtdd/code]
---
# summarise

#rtdd/code

## Tests

- [[bench/tests/test_falsesignal.py/test_summarise_reports_fired_silent_and_all_three_rates]] (calls)
- [[bench/tests/test_falsesignal.py/test_every_published_rate_refuses_to_pool_across_repos]] (calls)
- [[bench/tests/test_metrics.py/test_summarise_reports_both_recalls_the_duration_fraction_and_the_strata]] (calls)
- [[bench/tests/test_metrics.py/test_summarise_scores_only_the_named_strategy]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_round_2_recovers_a_failure_round_1_missed]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_the_rounds_table_puts_tdd_beside_both_arms]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_an_arm_missing_from_the_run_is_absent_not_imputed]] (calls)

## Callers

- [[bench/replay/report.py/build_summary]] (calls)

## Callees

- [[bench/replay/metrics.py/Ratio/to_dict]] (calls)
- [[bench/replay/metrics.py/assert_single_repo]] (calls)
- [[bench/replay/metrics.py/pair_up]] (calls)
- [[bench/replay/metrics.py/_detecting]] (calls)
- [[bench/replay/metrics.py/change_level_recall]] (calls)
- [[bench/replay/metrics.py/test_level_recall_micro]] (calls)
- [[bench/replay/metrics.py/test_level_recall_macro]] (calls)
- [[bench/replay/metrics.py/by_stratum]] (calls)
- [[bench/replay/metrics.py/selection_ratio]] (calls)
- [[bench/replay/metrics.py/selected_duration_fraction]] (calls)
- [[bench/replay/metrics.py/escalation_rate]] (calls)
