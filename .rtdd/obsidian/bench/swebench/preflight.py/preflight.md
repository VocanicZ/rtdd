---
id: "bench/swebench/preflight.py::preflight"
source: "bench/swebench/preflight.py"
start: 21
end: 43
kind: "func"
is-test: false
tags: [rtdd/code]
---
# preflight

#rtdd/code

## Tests

- [[bench/swebench/tests/test_acceptance.py/test_a_signed_tagged_and_matching_pre_registration_is_the_one_state_that_passes]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_signed_tagged_and_matching_passes]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_missing_instance_list_is_a_clean_refusal]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_instance_list_sha_mismatch_is_refused]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_instance_count_mismatch_is_refused]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_unsigned_prereg_is_refused_by_preflight]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_untagged_prereg_is_refused_by_preflight]] (calls)
- [[bench/swebench/tests/test_sample.py/test_preflight_passes_on_the_frozen_file]] (calls)
- [[bench/swebench/tests/test_sample.py/test_preflight_refuses_when_one_line_of_the_frozen_file_is_edited]] (calls)

## Callers

- [[bench/swebench/analyze.py/analyze]] (calls)
- [[bench/swebench/report.py/write_config]] (calls)
- [[bench/swebench/run_arm.py/run_arm]] (calls)
- [[bench/swebench/tests/test_acceptance.py/refusal]] (calls)

## Callees

- [[bench/PREREGISTRATION.md/_file]] (references)
- [[bench/swebench/instances.txt/_file]] (references)
- [[bench/swebench/preflight.py/instance_list_sha256]] (calls)
- [[bench/swebench/prereg.py/PreregError]] (calls)
- [[bench/swebench/prereg.py/load]] (calls)
- [[bench/swebench/prereg.py/assert_tagged]] (calls)
- [[bench/swebench/sample.py/_file]] (references)
