---
id: "bench/replay/falsesignal.py::summarise"
source: "bench/replay/falsesignal.py"
start: 84
end: 94
kind: "func"
is-test: false
tags: [rtdd/code]
---
# summarise

#rtdd/code

## Tests

- [[bench/tests/test_metrics.py/test_summarise_reports_both_recalls_the_duration_fraction_and_the_strata]] (calls)
- [[bench/tests/test_metrics.py/test_summarise_scores_only_the_named_strategy]] (calls)

## Callers

- [[bench/replay/report.py/build_summary]] (calls)

## Callees

- [[bench/replay/falsesignal.py/_fired]] (calls)
- [[bench/replay/falsesignal.py/change_false_signal_rate]] (calls)
- [[bench/replay/falsesignal.py/line_false_signal_rate]] (calls)
- [[bench/replay/falsesignal.py/fire_rate]] (calls)
- [[bench/replay/metrics.py/Ratio/to_dict]] (calls)
- [[bench/replay/metrics.py/assert_single_repo]] (calls)
- [[bench/replay/records.py/CommitRecord/to_dict]] (calls)
- [[bench/replay/records.py/StrategyRecord/to_dict]] (calls)
- [[bench/replay/records.py/WallClockRecord/to_dict]] (calls)
- [[bench/replay/records.py/UncoveredRecord/to_dict]] (calls)
