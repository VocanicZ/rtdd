---
id: "bench/replay/records.py::StrategyRecord"
source: "bench/replay/records.py"
start: 89
end: 142
kind: "class"
is-test: false
tags: [rtdd/code]
---
# StrategyRecord

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_derive_supersedes_a_stale_derived_record_rather_than_doubling_it]] (calls)
- [[bench/tests/test_records.py/test_jsonl_is_sorted_and_newline_terminated]] (calls)
- [[bench/tests/test_records.py/test_jsonl_orders_strategy_records_of_one_commit_by_strategy]] (calls)
- [[bench/tests/test_records.py/test_a_strategy_record_publishes_the_stale_ids_it_dropped]] (calls)
- [[bench/tests/test_records.py/test_strategy_record_defaults_to_measured]] (calls)
- [[bench/tests/test_records.py/test_a_derived_record_is_marked_on_the_wire]] (calls)

## Callers

- [[bench/replay/derive.py/derive_static]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/tests/test_cli.py/_published_repo]] (calls)
- [[bench/tests/test_cli.py/_published_with_importgraph]] (calls)
- [[bench/tests/test_derive.py/ig]] (calls)
- [[bench/tests/test_metrics.py/_sel]] (calls)
- [[bench/tests/test_records.py/_strategy_record]] (calls)
- [[bench/tests/test_report.py/_out]] (calls)
- [[bench/tests/test_report.py/_cheap_path_out]] (calls)
- [[bench/tests/test_report.py/_no_natural_detections]] (calls)
- [[bench/tests/test_report.py/_static_out]] (calls)
- [[bench/tests/test_strategy_rounds.py/_records/rec]] (calls)

## Callees

- [[bench/replay/records.py/StrategyRecord/to_dict]] (method)
- [[bench/replay/records.py/StrategyRecord/from_dict]] (method)
