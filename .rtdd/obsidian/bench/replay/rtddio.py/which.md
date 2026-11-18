---
id: "bench/replay/rtddio.py::which"
source: "bench/replay/rtddio.py"
start: 186
end: 188
kind: "func"
is-test: false
tags: [rtdd/code]
---
# which

#rtdd/code

## Tests

- [[bench/tests/test_rtddio.py/test_which_raises_when_the_binary_is_missing]] (calls)
- [[bench/tests/test_rtddio.py/test_which_raises_on_non_json_stdout]] (calls)
- [[bench/tests/test_rtddio.py/test_which_raises_on_a_usage_exit_code]] (calls)
- [[bench/tests/test_rtddio.py/test_which_raises_on_exit_1]] (calls)
- [[bench/tests/test_rtddio.py/test_wall_ms_is_measured_not_reported]] (calls)
- [[bench/tests/test_rtddio.py/test_the_harness_passes_no_tuning_flags]] (calls)
- [[bench/tests/test_rtddio.py/test_real_binary_honours_the_consumed_schema]] (calls)

## Callers

- [[bench/replay/session.py/run_drift]] (calls)
- [[bench/replay/strategies/rtdd.py/Rtdd/select]] (calls)

## Callees

- [[bench/replay/rtddio.py/parse_which]] (calls)
- [[bench/replay/rtddio.py/_invoke]] (calls)
- [[bench/replay/rtddio.py/_decode]] (calls)
