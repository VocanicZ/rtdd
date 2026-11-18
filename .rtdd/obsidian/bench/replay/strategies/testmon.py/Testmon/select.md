---
id: "bench/replay/strategies/testmon.py::Testmon::select"
source: "bench/replay/strategies/testmon.py"
start: 73
end: 99
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_testmon.py/test_testmon_selects_only_the_impacted_test_after_a_source_edit]] (calls)
- [[bench/tests/test_strategy_testmon.py/test_missing_database_is_an_escalation]] (calls)

## Callers

- [[bench/replay/strategies/testmon.py/Testmon]] (method)

## Callees

- [[bench/replay/strategies/base.py/Selection]] (calls)
- [[bench/replay/strategies/testmon.py/testmon_collect]] (calls)
