---
id: "bench/replay/records.py::StrategyRecord::from_dict"
source: "bench/replay/records.py"
start: 130
end: 142
kind: "method"
is-test: false
tags: [rtdd/code]
---
# from_dict

#rtdd/code

## Tests

- [[bench/tests/test_records.py/test_a_strategy_record_publishes_the_stale_ids_it_dropped]] (calls)
- [[bench/tests/test_records.py/test_a_derived_record_is_marked_on_the_wire]] (calls)
- [[bench/tests/test_records.py/test_an_already_committed_line_reads_back_as_measured]] (calls)

## Callers

- [[bench/replay/records.py/StrategyRecord]] (method)
- [[bench/replay/records.py/from_dict]] (calls)
- [[bench/replay/records.py/parse_jsonl_lines]] (calls)

## Callees

none
