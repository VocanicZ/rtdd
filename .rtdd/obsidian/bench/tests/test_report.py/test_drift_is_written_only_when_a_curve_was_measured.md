---
id: "bench/tests/test_report.py::test_drift_is_written_only_when_a_curve_was_measured"
source: "bench/tests/test_report.py"
start: 298
end: 302
kind: "func"
is-test: true
tags: [rtdd/test]
---
# test_drift_is_written_only_when_a_curve_was_measured

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[bench/replay/report.py/write_results]] (calls)
- [[bench/replay/session.py/DriftPoint]] (calls)
- [[bench/replay/session.py/DriftCurve]] (calls)
- [[bench/results/flask/drift.json/_file]] (references)
- [[bench/results/httpie/drift.json/_file]] (references)
- [[bench/results/sqlfluff/drift.json/_file]] (references)
- [[bench/tests/test_report.py/_out]] (calls)
