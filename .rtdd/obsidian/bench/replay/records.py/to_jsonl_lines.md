---
id: "bench/replay/records.py::to_jsonl_lines"
source: "bench/replay/records.py"
start: 246
end: 250
kind: "func"
is-test: false
tags: [rtdd/code]
---
# to_jsonl_lines

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_derive_supersedes_a_stale_derived_record_rather_than_doubling_it]] (calls)
- [[bench/tests/test_records.py/test_jsonl_is_sorted_and_newline_terminated]] (calls)
- [[bench/tests/test_records.py/test_jsonl_orders_strategy_records_of_one_commit_by_strategy]] (calls)
- [[bench/tests/test_records.py/test_jsonl_keys_are_sorted_within_each_line]] (calls)
- [[bench/tests/test_records.py/test_jsonl_is_byte_identical_across_runs_and_input_order]] (calls)
- [[bench/tests/test_records.py/test_jsonl_line_ordering_is_stable_for_a_dict_field_shuffle]] (calls)
- [[bench/tests/test_records.py/test_json_round_trip_is_lossless]] (calls)
- [[bench/tests/test_records.py/test_parse_jsonl_lines_round_trips_a_mixed_file_in_written_order]] (calls)
- [[bench/tests/test_records.py/test_parse_jsonl_lines_skips_blank_lines]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_derive]] (calls)
- [[bench/replay/report.py/write_results]] (calls)
- [[bench/tests/test_cli.py/_published_repo]] (calls)
- [[bench/tests/test_cli.py/_published_with_importgraph]] (calls)

## Callees

- [[bench/replay/records.py/CommitRecord/to_dict]] (calls)
- [[bench/replay/records.py/StrategyRecord/to_dict]] (calls)
- [[bench/replay/records.py/WallClockRecord/to_dict]] (calls)
- [[bench/replay/records.py/UncoveredRecord/to_dict]] (calls)
