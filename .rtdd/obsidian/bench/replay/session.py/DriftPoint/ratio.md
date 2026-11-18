---
id: "bench/replay/session.py::DriftPoint::ratio"
source: "bench/replay/session.py"
start: 55
end: 58
kind: "method"
is-test: false
tags: [rtdd/code]
---
# ratio

#rtdd/code

## Tests

- [[bench/tests/test_session.py/test_ratio_and_serialisation]] (calls)
- [[bench/tests/test_session.py/test_ratio_of_an_empty_suite_is_zero_not_a_zero_division]] (calls)
- [[bench/tests/test_session.py/test_drift_denominator_grows_with_the_tests_the_cycles_add]] (calls)
- [[bench/tests/test_session.py/test_a_cycle_whose_tree_will_not_collect_publishes_no_ratio]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_session]] (calls)
- [[bench/replay/session.py/DriftPoint]] (method)
- [[bench/replay/session.py/DriftPoint/to_dict]] (calls)

## Callees

none
