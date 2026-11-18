---
id: "bench/replay/cache.py::Cache::_complete_marker"
source: "bench/replay/cache.py"
start: 55
end: 59
kind: "method"
is-test: false
tags: [rtdd/code]
---
# _complete_marker

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_replaced_by_the_next_store]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/cache.py/Cache/artifact]] (calls)
- [[bench/replay/cache.py/Cache/store_artifact]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/_artifact_path]] (calls)
