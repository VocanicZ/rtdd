---
id: "bench/swebench/sample.py::draw"
source: "bench/swebench/sample.py"
start: 43
end: 47
kind: "func"
is-test: false
tags: [rtdd/code]
---
# draw

#rtdd/code

## Tests

- [[bench/swebench/tests/test_sample.py/test_draw_is_deterministic]] (calls)
- [[bench/swebench/tests/test_sample.py/test_draw_changes_with_seed]] (calls)
- [[bench/swebench/tests/test_sample.py/test_draw_never_returns_an_ineligible_instance]] (calls)
- [[bench/swebench/tests/test_sample.py/test_draw_refuses_when_the_eligible_pool_is_too_small]] (calls)

## Callers

none

## Callees

- [[bench/swebench/sample.py/eligible_ids]] (calls)
