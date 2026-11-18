---
id: "bench/replay/strategies/xdist.py::Xdist::select"
source: "bench/replay/strategies/xdist.py"
start: 27
end: 33
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_xdist.py/test_xdist_selects_everything_but_carries_parallel_exec_args]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_xdist_selects_the_whole_collected_suite_of_the_synthetic_repo]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_the_xdist_selections_exec_args_reach_the_pytest_invocation]] (calls)

## Callers

- [[bench/replay/strategies/xdist.py/Xdist]] (method)

## Callees

- [[bench/replay/strategies/base.py/Selection]] (calls)
