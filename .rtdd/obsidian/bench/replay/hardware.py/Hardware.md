---
id: "bench/replay/hardware.py::Hardware"
source: "bench/replay/hardware.py"
start: 60
end: 99
kind: "class"
is-test: false
tags: [rtdd/code]
---
# Hardware

#rtdd/code

## Tests

- [[bench/tests/test_hardware.py/test_fingerprint_is_stable_and_excludes_ci_flag]] (calls)

## Callers

- [[bench/replay/hardware.py/probe]] (calls)
- [[bench/tests/test_cli.py/_published_repo]] (calls)

## Callees

- [[bench/replay/hardware.py/Hardware/to_dict]] (method)
- [[bench/replay/hardware.py/Hardware/from_dict]] (method)
- [[bench/replay/hardware.py/Hardware/fingerprint]] (method)
- [[bench/replay/hardware.py/Hardware/wallclock_allowed]] (method)
