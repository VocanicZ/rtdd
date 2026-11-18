---
id: "bench/replay/cache.py::Cache::key"
source: "bench/replay/cache.py"
start: 43
end: 47
kind: "method"
is-test: false
tags: [rtdd/code]
---
# key

#rtdd/code

## Tests

- [[bench/tests/test_cache.py/test_key_includes_config_digest_so_a_config_change_misses]] (calls)
- [[bench/tests/test_cache.py/test_key_distinguishes_every_coordinate_of_the_tuple]] (calls)
- [[bench/tests/test_cache.py/test_key_is_not_confusable_by_concatenating_adjacent_parts]] (calls)
- [[bench/tests/test_cache.py/test_a_config_change_is_a_miss_not_a_silent_mix_of_old_and_new_numbers]] (calls)
- [[bench/tests/test_cache.py/test_json_round_trip_and_build_is_called_once]] (calls)
- [[bench/tests/test_cache.py/test_json_survives_a_fresh_cache_object_on_the_same_root]] (calls)
- [[bench/tests/test_cache.py/test_artifact_round_trip]] (calls)
- [[bench/tests/test_cache.py/test_a_restored_artifact_is_byte_identical_to_what_was_stored]] (calls)
- [[bench/tests/test_cache.py/test_restoring_a_missing_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_artifact_hit_and_miss_are_counted]] (calls)
- [[bench/tests/test_cache.py/test_storing_an_artifact_twice_replaces_the_old_bytes]] (calls)
- [[bench/tests/test_cache.py/test_a_corrupt_json_entry_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_truncated_or_wrong_shaped_json_entry_is_a_miss]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_a_miss_not_a_crash]] (calls)
- [[bench/tests/test_cache.py/test_a_partially_written_artifact_is_replaced_by_the_next_store]] (calls)
- [[bench/tests/test_cache.py/test_a_leftover_staging_directory_is_never_mistaken_for_an_entry]] (calls)
- [[bench/tests/test_cache.py/test_the_cache_writes_only_under_its_own_root]] (calls)

## Callers

- [[bench/replay/cache.py/Cache]] (method)
- [[bench/replay/replay.py/_prepare_state]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/replay/runner.py/run_key]] (calls)

## Callees

none
