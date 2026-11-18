---
id: "bench/replay/cache.py::Cache::_json_path"
source: "bench/replay/cache.py"
start: 49
end: 50
kind: "method"
is-test: false
tags: [rtdd/code]
---
# _json_path

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_a_corrupt_json_entry_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_truncated_or_wrong_shaped_json_entry_is_a_miss]] (calls)
- [[bench/tests/test_cache.py/test_the_cache_writes_only_under_its_own_root]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/cache.py/Cache/get_json]] (calls)
- [[bench/replay/cache.py/Cache/put_json]] (calls)

## Callees

none
