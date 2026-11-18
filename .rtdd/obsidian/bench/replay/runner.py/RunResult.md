---
id: "bench/replay/runner.py::RunResult"
source: "bench/replay/runner.py"
start: 76
end: 90
kind: "class"
is-test: false
tags: [rtdd/code]
---
# RunResult

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_run_subset_of_nothing_never_invokes_pytest]] (calls)
- [[bench/tests/test_runner.py/test_a_run_result_round_trips_through_the_cache_payload]] (calls)

## Callers

- [[bench/replay/runner.py/_invoke]] (calls)
- [[bench/replay/runner.py/run_subset]] (calls)
- [[bench/replay/runner.py/result_from_dict]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed/build]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit/build]] (calls)

## Callees

- [[bench/replay/runner.py/RunResult/failing]] (method)
- [[bench/replay/runner.py/RunResult/durations]] (method)
