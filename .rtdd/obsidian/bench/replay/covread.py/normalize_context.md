---
id: "bench/replay/covread.py::normalize_context"
source: "bench/replay/covread.py"
start: 62
end: 77
kind: "func"
is-test: false
tags: [rtdd/code]
---
# normalize_context

#rtdd/code

## Tests

- [[bench/tests/test_covread.py/test_normalize_context_strips_the_phase_suffix]] (calls)
- [[bench/tests/test_covread.py/test_normalize_context_splits_on_the_last_pipe_only]] (calls)
- [[bench/tests/test_covread.py/test_a_static_context_with_no_phase_suffix_is_still_a_test]] (calls)
- [[bench/tests/test_covread.py/test_a_phase_suffix_with_no_test_id_is_not_a_test]] (calls)

## Callers

- [[bench/replay/covread.py/read_coverage]] (calls)

## Callees

none
