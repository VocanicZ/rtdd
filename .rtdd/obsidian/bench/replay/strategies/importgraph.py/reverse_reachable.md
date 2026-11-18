---
id: "bench/replay/strategies/importgraph.py::reverse_reachable"
source: "bench/replay/strategies/importgraph.py"
start: 91
end: 105
kind: "func"
is-test: false
tags: [rtdd/code]
---
# reverse_reachable

#rtdd/code

## Tests

- [[bench/tests/test_strategy_importgraph.py/test_reverse_reachable_is_transitive]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_reverse_reachable_terminates_on_an_import_cycle]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_a_change_reaches_tests_transitively_through_a_package]] (calls)

## Callers

- [[bench/replay/strategies/importgraph.py/ImportGraph/select]] (calls)

## Callees

- [[bench/replay/strategies/base.py/get]] (calls)
