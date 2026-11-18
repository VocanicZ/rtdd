---
id: "bench/replay/runner.py::parse_report_log"
source: "bench/replay/runner.py"
start: 102
end: 147
kind: "func"
is-test: false
tags: [rtdd/code]
---
# parse_report_log

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_report_log_parsing_yields_pass_fail_skip_and_error]] (calls)
- [[bench/tests/test_runner.py/test_report_log_parsing_sums_every_phase_into_one_duration]] (calls)
- [[bench/tests/test_runner.py/test_a_teardown_failure_after_a_passing_call_is_an_error]] (calls)
- [[bench/tests/test_runner.py/test_a_collection_error_is_a_recorded_outcome]] (calls)
- [[bench/tests/test_runner.py/test_a_collect_report_never_overwrites_a_real_test_outcome]] (calls)
- [[bench/tests/test_runner.py/test_parse_report_log_ignores_blank_lines_and_other_record_types]] (calls)

## Callers

- [[bench/replay/runner.py/_invoke]] (calls)

## Callees

- [[bench/replay/runner.py/Outcome]] (calls)
