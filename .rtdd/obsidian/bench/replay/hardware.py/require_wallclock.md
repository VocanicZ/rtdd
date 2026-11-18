---
id: "bench/replay/hardware.py::require_wallclock"
source: "bench/replay/hardware.py"
start: 114
end: 120
kind: "func"
is-test: false
tags: [rtdd/code]
---
# require_wallclock

#rtdd/code

## Tests

- [[bench/tests/test_hardware.py/test_require_wallclock_refuses_on_ci_and_allows_on_a_disclosed_machine]] (calls)

## Callers

- [[bench/replay/cli.py/_wallclock_enabled]] (calls)

## Callees

- [[bench/replay/hardware.py/CIWallClockRefused]] (calls)
- [[bench/replay/hardware.py/Hardware/wallclock_allowed]] (calls)
