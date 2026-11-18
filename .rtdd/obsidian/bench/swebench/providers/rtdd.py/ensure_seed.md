---
id: "bench/swebench/providers/rtdd.py::ensure_seed"
source: "bench/swebench/providers/rtdd.py"
start: 140
end: 169
kind: "func"
is-test: false
tags: [rtdd/code]
---
# ensure_seed

#rtdd/code

## Tests

- [[bench/swebench/tests/test_providers_rtdd.py/test_ensure_seed_runs_the_suite_in_the_instance_image_and_caches_the_map]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_cached_seed_is_reused_without_re_running_the_suite]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_the_second_arm_shares_the_first_arms_seed]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_failed_seed_is_recorded_and_the_instance_is_not_dropped]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_an_empty_map_from_a_zero_exit_run_is_a_seed_failure]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_timed_out_seed_returns_a_reason_rather_than_raising]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_cached_failure_is_not_retried]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_missing_rtdd_binary_is_raised_not_published_as_a_seed_failure]] (calls)

## Callers

- [[bench/swebench/providers/__init__.py/context_tools]] (calls)

## Callees

- [[bench/swebench/providers/rtdd.py/_rtdd_path]] (calls)
- [[bench/swebench/providers/rtdd.py/seed_cache_key]] (calls)
- [[bench/swebench/providers/rtdd.py/_seed_in_image]] (calls)
- [[bench/swebench/providers/rtdd.py/_write_marker]] (calls)
- [[docs/results/worked-example/map.jsonl/_file]] (references)
