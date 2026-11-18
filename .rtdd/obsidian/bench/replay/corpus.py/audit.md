---
id: "bench/replay/corpus.py::audit"
source: "bench/replay/corpus.py"
start: 205
end: 277
kind: "func"
is-test: false
tags: [rtdd/code]
---
# audit

#rtdd/code

## Tests

- [[bench/tests/test_audit.py/test_the_sqlfluff_breach_v1_never_caught]] (calls)
- [[bench/tests/test_audit.py/test_a_repo_under_the_collected_test_floor_is_a_breach]] (calls)
- [[bench/tests/test_audit.py/test_a_compiled_extension_on_the_import_path_is_a_breach]] (calls)
- [[bench/tests/test_audit.py/test_an_unmeasured_repo_is_a_breach_not_a_pass]] (calls)
- [[bench/tests/test_audit.py/test_a_partially_measured_repo_is_a_breach_per_missing_measurement]] (calls)
- [[bench/tests/test_audit.py/test_declaring_more_depth_than_was_reachable_is_a_breach]] (calls)
- [[bench/tests/test_audit.py/test_a_corpus_with_no_thresholds_can_decide_nothing]] (calls)
- [[bench/tests/test_audit.py/test_the_shipped_corpus_passes_its_own_audit]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_doctor]] (calls)
- [[bench/replay/cli.py/cmd_audit]] (calls)

## Callees

- [[bench/replay/corpus.py/Corpus/ids]] (calls)
- [[bench/replay/corpus.py/Finding]] (calls)
