---
id: "bench/replay/cache.py::Cache::json_or_build"
source: "bench/replay/cache.py"
start: 86
end: 94
kind: "method"
is-test: false
tags: [rtdd/code]
---
# json_or_build

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once]] (calls)
- [[bench/tests/test_cache.py/test_a_corrupt_json_entry_is_a_miss_not_a_crash]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/replay.py/_cached_selection]] (calls)
- [[bench/replay/replay.py/_cached_collect]] (calls)
- [[bench/replay/runner.py/cached_run]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/get_json]] (calls)
- [[bench/replay/cache.py/Cache/put_json]] (calls)
- [[bench/swebench/prompts.py/build]] (calls)
- [[bench/swebench/tests/conftest.py/make_repo/build]] (calls)
- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once/build]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed/build]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit/build]] (calls)
