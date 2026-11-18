---
id: "bench/replay/session.py::DriftCurve::to_dict"
source: "bench/replay/session.py"
start: 80
end: 85
kind: "method"
is-test: false
tags: [rtdd/code]
---
# to_dict

#rtdd/code

## Tests

- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)
- [[bench/tests/test_session.py/test_a_cycle_whose_tree_will_not_collect_publishes_no_ratio]] (calls)

## Callers

- [[bench/replay/report.py/write_results]] (calls)
- [[bench/replay/session.py/DriftCurve]] (method)

## Callees

- [[bench/replay/session.py/DriftPoint/to_dict]] (calls)
