---
id: "bench/replay/runner.py::run_key"
source: "bench/replay/runner.py"
start: 335
end: 362
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run_key

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_a_cached_run_is_computed_once_and_replayed]] (calls)
- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit]] (calls)
- [[bench/tests/test_runner.py/test_run_keys_separate_the_three_modes_at_the_same_commit]] (calls)
- [[bench/tests/test_runner.py/test_a_subset_run_key_depends_on_the_selected_tests_not_their_order]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_of_the_real_suite_skips_the_second_pytest]] (calls)
- [[bench/tests/test_runner.py/test_a_parallel_run_key_is_not_the_serial_one]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/cache.py/Cache/key]] (calls)
