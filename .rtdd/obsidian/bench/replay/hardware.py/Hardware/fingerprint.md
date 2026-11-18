---
id: "bench/replay/hardware.py::Hardware::fingerprint"
source: "bench/replay/hardware.py"
start: 91
end: 96
kind: "method"
is-test: false
tags: [rtdd/code]
---
# fingerprint

#rtdd/code

## Tests

- [[bench/tests/test_hardware.py/test_fingerprint_is_stable_and_excludes_ci_flag]] (calls)
- [[bench/tests/test_replay.py/test_wall_clock_captures_all_three_forms_and_the_isolation_check]] (calls)
- [[bench/tests/test_replay.py/test_a_cached_run_replayed_under_different_hardware_does_not_publish_borrowed_timing]] (calls)
- [[bench/tests/test_report.py/test_config_json_carries_everything_needed_to_reproduce_the_numbers]] (calls)

## Callers

- [[bench/replay/hardware.py/Hardware]] (method)
- [[bench/replay/hardware.py/Hardware/to_dict]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/replay/report.py/render_markdown]] (calls)
- [[bench/tests/test_report.py/_out]] (calls)
- [[bench/tests/test_report.py/_spread_out]] (calls)

## Callees

none
