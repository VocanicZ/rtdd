---
id: "bench/replay/cache.py::Cache::put_json"
source: "bench/replay/cache.py"
start: 73
end: 84
kind: "method"
is-test: false
tags: [rtdd/code]
---
# put_json

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_a_config_change_is_a_miss_not_a_silent_mix_of_old_and_new_numbers]] (calls)
- [[bench/tests/test_cache.py/test_json_survives_a_fresh_cache_object_on_the_same_root]] (calls)
- [[bench/tests/test_cache.py/test_a_corrupt_json_entry_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_truncated_or_wrong_shaped_json_entry_is_a_miss]] (calls)
- [[bench/tests/test_cache.py/test_the_cache_writes_only_under_its_own_root]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/cache.py/Cache/json_or_build]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/_json_path]] (calls)
- [[bench/replay/config.py/canonical_json]] (calls)
