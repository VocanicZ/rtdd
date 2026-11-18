---
id: "bench/replay/rtddio.py::parse_which"
source: "bench/replay/rtddio.py"
start: 112
end: 127
kind: "func"
is-test: false
tags: [rtdd/code]
---
# parse_which

#rtdd/code

## Tests

- [[bench/tests/test_rtddio.py/test_parse_which_reads_rounds_one_and_two]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_rejects_any_schema_but_3]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_rejects_a_document_with_no_schema_key]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_rejects_a_schema_2_document]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_rejects_a_document_with_no_rounds]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_rejects_a_missing_round]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_rejects_a_test_with_no_id]] (calls)
- [[bench/tests/test_rtddio.py/test_parse_which_does_not_need_round_3]] (calls)

## Callers

- [[bench/replay/rtddio.py/which]] (calls)

## Callees

- [[bench/replay/rtddio.py/RtddError]] (calls)
- [[bench/replay/rtddio.py/WhichResult]] (calls)
- [[bench/replay/rtddio.py/_require_schema]] (calls)
- [[bench/replay/rtddio.py/_require]] (calls)
- [[bench/replay/rtddio.py/_list]] (calls)
- [[bench/replay/rtddio.py/_strings]] (calls)
- [[bench/replay/rtddio.py/_changed_paths]] (calls)
- [[bench/replay/rtddio.py/_round_tests]] (calls)
