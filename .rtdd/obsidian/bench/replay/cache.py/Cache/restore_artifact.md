---
id: "bench/replay/cache.py::Cache::restore_artifact"
source: "bench/replay/cache.py"
start: 121
end: 134
kind: "method"
is-test: false
tags: [rtdd/code]
---
# restore_artifact

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_artifact_round_trip]] (calls)
- [[bench/tests/test_cache.py/test_a_restored_artifact_is_byte_identical_to_what_was_stored]] (calls)
- [[bench/tests/test_cache.py/test_restoring_a_missing_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_artifact_hit_and_miss_are_counted]] (calls)
- [[bench/tests/test_cache.py/test_storing_an_artifact_twice_replaces_the_old_bytes]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_replaced_by_the_next_store]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/replay.py/_prepare_state]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/artifact]] (calls)
