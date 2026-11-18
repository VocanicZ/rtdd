---
id: "bench/replay/cache.py::Cache::artifact"
source: "bench/replay/cache.py"
start: 96
end: 100
kind: "method"
is-test: false
tags: [rtdd/code]
---
# artifact

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_artifact_round_trip]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_a_miss_not_a_crash]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/cache.py/Cache/restore_artifact]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/_artifact_path]] (calls)
- [[bench/replay/cache.py/Cache/_complete_marker]] (calls)
