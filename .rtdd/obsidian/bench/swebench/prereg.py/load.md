---
id: "bench/swebench/prereg.py::load"
source: "bench/swebench/prereg.py"
start: 50
end: 79
kind: "func"
is-test: false
tags: [rtdd/code]
---
# load

#rtdd/code

## Tests

- [[bench/swebench/tests/test_prereg.py/test_complete_and_signed_loads]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_unsigned_is_refused]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_missing_kill_criterion_is_refused]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_blank_kill_criterion_is_refused]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_placeholder_kill_criterion_is_refused]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_no_front_matter_is_refused]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_missing_file_is_refused]] (calls)
- [[bench/swebench/tests/test_prereg.py/test_shipped_preregistration_is_unsigned_and_has_no_kill_criterion]] (calls)

## Callers

- [[bench/swebench/analyze.py/kill_criterion_met]] (calls)
- [[bench/swebench/preflight.py/preflight]] (calls)

## Callees

- [[bench/swebench/prereg.py/PreregError]] (calls)
- [[bench/swebench/prereg.py/Prereg]] (calls)
