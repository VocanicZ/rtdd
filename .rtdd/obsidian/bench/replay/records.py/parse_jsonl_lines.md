---
id: "bench/replay/records.py::parse_jsonl_lines"
source: "bench/replay/records.py"
start: 253
end: 255
kind: "func"
is-test: false
tags: [rtdd/code]
---
# parse_jsonl_lines

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_derive_appends_a_static_record_per_commit]] (calls)
- [[bench/tests/test_cli.py/test_derive_supersedes_a_stale_derived_record_rather_than_doubling_it]] (calls)
- [[bench/tests/test_cli.py/test_derive_leaves_the_measured_records_exactly_as_the_run_wrote_them]] (calls)
- [[bench/tests/test_records.py/test_json_round_trip_is_lossless]] (calls)
- [[bench/tests/test_records.py/test_parse_jsonl_lines_round_trips_a_mixed_file_in_written_order]] (calls)
- [[bench/tests/test_records.py/test_parse_jsonl_lines_skips_blank_lines]] (calls)
- [[bench/tests/test_results_static_arm.py/test_the_committed_records_carry_one_derived_static_record_per_commit]] (calls)

## Callers

- [[bench/replay/cli.py/_rebuild_repo]] (calls)
- [[bench/replay/cli.py/cmd_derive]] (calls)
- [[bench/tests/test_cli.py/_published_with_importgraph]] (calls)

## Callees

- [[bench/replay/records.py/CommitRecord/from_dict]] (calls)
- [[bench/replay/records.py/StrategyRecord/from_dict]] (calls)
- [[bench/replay/records.py/WallClockRecord/from_dict]] (calls)
- [[bench/replay/records.py/UncoveredRecord/from_dict]] (calls)
- [[bench/replay/records.py/from_dict]] (calls)
