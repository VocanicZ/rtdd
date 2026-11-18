---
id: "bench/replay/config.py::canonical_json"
source: "bench/replay/config.py"
start: 20
end: 21
kind: "func"
is-test: false
tags: [rtdd/code]
---
# canonical_json

#rtdd/code

## Tests

- [[bench/tests/test_config.py/test_canonical_json_is_key_order_independent_and_newline_terminated]] (calls)
- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)

## Callers

- [[bench/replay/cache.py/Cache/put_json]] (calls)
- [[bench/replay/cli.py/cmd_session]] (calls)
- [[bench/replay/cli.py/_rebuild_repo]] (calls)
- [[bench/replay/config.py/RunConfig/digest]] (calls)
- [[bench/replay/report.py/write_results]] (calls)

## Callees

none
