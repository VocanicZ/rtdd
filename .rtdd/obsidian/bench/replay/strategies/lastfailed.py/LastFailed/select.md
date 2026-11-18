---
id: "bench/replay/strategies/lastfailed.py::LastFailed::select"
source: "bench/replay/strategies/lastfailed.py"
start: 59
end: 72
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_lastfailed.py/test_selects_only_previously_failing_tests]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/test_empty_cache_degenerates_to_the_full_suite_and_is_an_escalation]] (calls)

## Callers

- [[bench/replay/strategies/lastfailed.py/LastFailed]] (method)

## Callees

- [[bench/replay/strategies/base.py/Selection]] (calls)
- [[bench/replay/strategies/lastfailed.py/read_lastfailed]] (calls)
