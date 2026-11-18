---
id: "bench/replay/rtddio.py::rtdd_version"
source: "bench/replay/rtddio.py"
start: 154
end: 174
kind: "func"
is-test: false
tags: [rtdd/code]
---
# rtdd_version

#rtdd/code

## Tests

- [[bench/tests/test_rtddio.py/test_rtdd_version_prefers_the_binarys_own_version_output]] (calls)
- [[bench/tests/test_rtddio.py/test_rtdd_version_falls_back_to_a_content_digest]] (calls)
- [[bench/tests/test_rtddio.py/test_rtdd_version_raises_when_the_binary_is_missing]] (calls)

## Callers

- [[bench/replay/cli.py/_config]] (calls)
- [[bench/replay/cli.py/cmd_doctor]] (calls)

## Callees

- [[bench/replay/rtddio.py/RtddError]] (calls)
