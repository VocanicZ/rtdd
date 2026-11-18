---
id: "bench/replay/config.py::RunConfig::to_dict"
source: "bench/replay/config.py"
start: 46
end: 57
kind: "method"
is-test: false
tags: [rtdd/code]
---
# to_dict

#rtdd/code

## Tests

- [[bench/tests/test_config.py/test_to_dict_carries_every_published_knob]] (calls)
- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)
- [[bench/tests/test_session.py/test_a_cycle_whose_tree_will_not_collect_publishes_no_ratio]] (calls)

## Callers

- [[bench/replay/config.py/RunConfig]] (method)
- [[bench/replay/config.py/RunConfig/digest]] (calls)
- [[bench/replay/report.py/write_results]] (calls)
- [[bench/tests/test_cli.py/_published_repo]] (calls)

## Callees

none
