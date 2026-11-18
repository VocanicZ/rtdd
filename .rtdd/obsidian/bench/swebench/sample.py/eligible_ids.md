---
id: "bench/swebench/sample.py::eligible_ids"
source: "bench/swebench/sample.py"
start: 39
end: 40
kind: "func"
is-test: false
tags: [rtdd/code]
---
# eligible_ids

#rtdd/code

## Tests

- [[bench/swebench/tests/test_sample.py/test_empty_p2p_is_ineligible]] (calls)
- [[bench/swebench/tests/test_sample.py/test_eligible_is_sorted]] (calls)
- [[bench/swebench/tests/test_sample.py/test_eligible_accepts_a_list_valued_p2p_column]] (calls)
- [[bench/swebench/tests/test_sample.py/test_draw_never_returns_an_ineligible_instance]] (calls)

## Callers

- [[bench/swebench/sample.py/draw]] (calls)

## Callees

- [[bench/swebench/sample.py/_p2p]] (calls)
