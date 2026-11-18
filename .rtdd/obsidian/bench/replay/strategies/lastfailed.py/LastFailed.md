---
id: "bench/replay/strategies/lastfailed.py::LastFailed"
source: "bench/replay/strategies/lastfailed.py"
start: 46
end: 72
kind: "class"
is-test: false
tags: [rtdd/code]
---
# LastFailed

#rtdd/code

## Tests

- [[bench/tests/test_strategy_lastfailed.py/test_selects_only_previously_failing_tests]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/test_empty_cache_degenerates_to_the_full_suite_and_is_an_escalation]] (calls)
- [[bench/tests/test_strategy_lastfailed.py/test_prepare_seeds_the_cache_in_the_parent_tree]] (calls)

## Callers

none

## Callees

- [[bench/replay/strategies/lastfailed.py/LastFailed/prepare]] (method)
- [[bench/replay/strategies/lastfailed.py/LastFailed/select]] (method)
