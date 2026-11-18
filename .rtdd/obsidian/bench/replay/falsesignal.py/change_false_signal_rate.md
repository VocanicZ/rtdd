---
id: "bench/replay/falsesignal.py::change_false_signal_rate"
source: "bench/replay/falsesignal.py"
start: 63
end: 67
kind: "func"
is-test: false
tags: [rtdd/code]
---
# change_false_signal_rate

#rtdd/code

## Tests

- [[bench/tests/test_falsesignal.py/test_change_level_false_signal_requires_every_reported_line_to_be_tested]] (calls)
- [[bench/tests/test_falsesignal.py/test_silent_commits_are_excluded_from_the_rate_but_counted_in_fire_rate]] (calls)
- [[bench/tests/test_falsesignal.py/test_a_rate_with_no_fired_commit_has_no_value_rather_than_a_zero]] (calls)
- [[bench/tests/test_falsesignal.py/test_every_published_rate_refuses_to_pool_across_repos]] (calls)
- [[bench/tests/test_falsesignal.py/test_false_signal_rate_against_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/falsesignal.py/summarise]] (calls)

## Callees

- [[bench/replay/falsesignal.py/_fired]] (calls)
- [[bench/replay/metrics.py/Ratio]] (calls)
