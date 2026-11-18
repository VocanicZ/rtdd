---
id: "bench/tests/test_strategy_importgraph.py::test_a_change_reaches_tests_transitively_through_a_package"
source: "bench/tests/test_strategy_importgraph.py"
start: 101
end: 107
kind: "func"
is-test: true
tags: [rtdd/test]
---
# test_a_change_reaches_tests_transitively_through_a_package

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[bench/replay/__init__.py/_file]] (references)
- [[bench/replay/strategies/__init__.py/_file]] (references)
- [[bench/replay/strategies/importgraph.py/build_graph]] (calls)
- [[bench/replay/strategies/importgraph.py/reverse_reachable]] (calls)
- [[bench/swebench/__init__.py/_file]] (references)
- [[bench/swebench/providers/__init__.py/_file]] (references)
- [[bench/tests/test_strategy_importgraph.py/_write]] (calls)
