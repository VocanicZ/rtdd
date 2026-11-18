---
id: "bench/replay/runner.py::cached_run"
source: "bench/replay/runner.py"
start: 365
end: 369
kind: "func"
is-test: false
tags: [rtdd/code]
---
# cached_run

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_of_the_real_suite_skips_the_second_pytest]] (calls)

## Callers

- [[bench/replay/replay.py/_cached_full]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/json_or_build]] (calls)
- [[bench/replay/cache.py/Cache/stats]] (calls)
- [[bench/replay/runner.py/result_to_dict]] (calls)
- [[bench/replay/runner.py/result_from_dict]] (calls)
- [[bench/swebench/prompts.py/build]] (calls)
- [[bench/swebench/tests/conftest.py/make_repo/build]] (calls)
- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once/build]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed/build]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit/build]] (calls)
