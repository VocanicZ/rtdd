---
id: "bench/replay/runner.py::collect"
source: "bench/replay/runner.py"
start: 172
end: 205
kind: "func"
is-test: false
tags: [rtdd/code]
---
# collect

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_collect_lists_every_test_id]] (calls)
- [[bench/tests/test_runner.py/test_collect_ignores_node_ids_quoted_inside_a_warning]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_it_samples_real_ids_from_the_synthetic_repo]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_xdist_selects_the_whole_collected_suite_of_the_synthetic_repo]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_the_xdist_selections_exec_args_reach_the_pytest_invocation]] (calls)

## Callers

- [[bench/replay/replay.py/_cached_collect]] (calls)
- [[bench/replay/session.py/run_drift]] (calls)

## Callees

- [[bench/replay/runner.py/_env]] (calls)
