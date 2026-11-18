---
id: "bench/replay/falsesignal.py::line_false_signal_rate"
source: "bench/replay/falsesignal.py"
start: 70
end: 76
kind: "func"
is-test: false
tags: [rtdd/code]
---
# line_false_signal_rate

#rtdd/code

## Tests

- [[bench/tests/test_falsesignal.py/test_line_level_false_signal_rate]] (calls)
- [[bench/tests/test_falsesignal.py/test_a_rate_with_no_fired_commit_has_no_value_rather_than_a_zero]] (calls)
- [[bench/tests/test_falsesignal.py/test_every_published_rate_refuses_to_pool_across_repos]] (calls)
- [[bench/tests/test_falsesignal.py/test_false_signal_rate_against_the_synthetic_repo]] (calls)
- [[bench/tests/test_falsesignal.py/test_truth_is_only_the_instrumented_run_never_the_reported_set]] (calls)

## Callers

- [[bench/replay/falsesignal.py/summarise]] (calls)

## Callees

- [[bench/replay/falsesignal.py/_fired]] (calls)
- [[bench/replay/metrics.py/Ratio]] (calls)
