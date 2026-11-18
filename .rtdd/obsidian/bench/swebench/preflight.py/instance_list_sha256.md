---
id: "bench/swebench/preflight.py::instance_list_sha256"
source: "bench/swebench/preflight.py"
start: 16
end: 18
kind: "func"
is-test: false
tags: [rtdd/code]
---
# instance_list_sha256

#rtdd/code

## Tests

- [[bench/swebench/tests/test_acceptance.py/test_the_shipped_pre_registration_is_complete_except_the_humans_number]] (calls)
- [[bench/swebench/tests/test_preflight.py/test_instance_list_sha256_ignores_blank_lines_and_trailing_whitespace]] (calls)
- [[bench/swebench/tests/test_report.py/test_config_records_the_sample_seed_size_and_instance_list_sha256]] (calls)
- [[bench/swebench/tests/test_report.py/test_the_published_digest_is_the_one_the_gate_checked]] (calls)
- [[bench/swebench/tests/test_sample.py/test_frozen_file_sha_is_reproducible_from_the_file_alone]] (calls)
- [[bench/swebench/tests/test_sample.py/test_prereg_carries_the_frozen_files_sha]] (calls)
- [[bench/swebench/tests/test_sample.py/test_preflight_passes_on_the_frozen_file]] (calls)
- [[bench/swebench/tests/test_sample.py/test_preflight_refuses_when_one_line_of_the_frozen_file_is_edited]] (calls)

## Callers

- [[bench/swebench/preflight.py/preflight]] (calls)
- [[bench/swebench/report.py/write_config]] (calls)

## Callees

none
