---
id: "bench/replay/derive.py::level1_tests"
source: "bench/replay/derive.py"
start: 120
end: 133
kind: "func"
is-test: false
tags: [rtdd/code]
---
# level1_tests

#rtdd/code

## Tests

- [[bench/tests/test_derive.py/test_a_changed_test_file_contributes_its_own_collected_ids]] (calls)
- [[bench/tests/test_derive.py/test_a_non_python_change_contributes_nothing]] (calls)
- [[bench/tests/test_derive.py/test_level1_expands_a_resolved_file_to_every_id_it_collected]] (calls)

## Callers

- [[bench/replay/derive.py/derive_static]] (calls)

## Callees

- [[bench/replay/derive.py/test_for_candidate]] (calls)
