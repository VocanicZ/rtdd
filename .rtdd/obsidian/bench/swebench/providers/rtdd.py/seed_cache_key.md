---
id: "bench/swebench/providers/rtdd.py::seed_cache_key"
source: "bench/swebench/providers/rtdd.py"
start: 86
end: 93
kind: "func"
is-test: false
tags: [rtdd/code]
---
# seed_cache_key

#rtdd/code

## Tests

- [[bench/swebench/tests/test_providers_rtdd.py/test_seed_cache_key_is_stable_across_calls]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_two_instances_sharing_a_base_commit_share_one_seed]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_different_base_commit_is_a_different_seed]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_different_repo_is_a_different_seed]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_the_key_is_content_addressed_not_name_addressed]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_ensure_seed_runs_the_suite_in_the_instance_image_and_caches_the_map]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_failed_seed_is_recorded_and_the_instance_is_not_dropped]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_timed_out_seed_returns_a_reason_rather_than_raising]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_missing_rtdd_binary_is_raised_not_published_as_a_seed_failure]] (calls)

## Callers

- [[bench/swebench/providers/rtdd.py/ensure_seed]] (calls)

## Callees

none
