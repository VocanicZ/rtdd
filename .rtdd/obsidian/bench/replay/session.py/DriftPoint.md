---
id: "bench/replay/session.py::DriftPoint"
source: "bench/replay/session.py"
start: 42
end: 69
kind: "class"
is-test: false
tags: [rtdd/code]
---
# DriftPoint

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_drift_is_written_only_when_a_curve_was_measured]] (calls)
- [[bench/tests/test_session.py/test_ratio_and_serialisation]] (calls)
- [[bench/tests/test_session.py/test_ratio_of_an_empty_suite_is_zero_not_a_zero_division]] (calls)

## Callers

- [[bench/replay/session.py/run_drift]] (calls)

## Callees

- [[bench/replay/session.py/DriftPoint/ratio]] (method)
- [[bench/replay/session.py/DriftPoint/to_dict]] (method)
