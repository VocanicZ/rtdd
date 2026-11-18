---
id: "bench/replay/cache.py::Cache::stats"
source: "bench/replay/cache.py"
start: 136
end: 137
kind: "method"
is-test: false
tags: [rtdd/code]
---
# stats

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once]] (calls)
- [[bench/tests/test_cache.py/test_restoring_a_missing_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_artifact_hit_and_miss_are_counted]] (calls)
- [[bench/tests/test_replay.py/test_a_second_replay_is_served_from_cache]] (calls)
- [[bench/tests/test_replay.py/test_a_config_change_is_a_cache_miss_not_a_silent_mix]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/runner.py/cached_run]] (calls)

## Callees

none
