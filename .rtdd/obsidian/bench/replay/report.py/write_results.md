---
id: "bench/replay/report.py::write_results"
source: "bench/replay/report.py"
start: 831
end: 860
kind: "func"
is-test: false
tags: [rtdd/code]
---
# write_results

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_write_results_is_byte_stable_across_reruns]] (calls)
- [[bench/tests/test_report.py/test_commits_jsonl_is_one_sorted_json_line_per_record]] (calls)
- [[bench/tests/test_report.py/test_summary_json_has_sorted_keys_and_a_trailing_newline]] (calls)
- [[bench/tests/test_report.py/test_config_json_carries_everything_needed_to_reproduce_the_numbers]] (calls)
- [[bench/tests/test_report.py/test_no_result_file_is_written_without_its_config]] (calls)
- [[bench/tests/test_report.py/test_drift_is_written_only_when_a_curve_was_measured]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_replay]] (calls)

## Callees

- [[bench/replay/config.py/canonical_json]] (calls)
- [[bench/replay/config.py/RunConfig/to_dict]] (calls)
- [[bench/replay/config.py/RunConfig/digest]] (calls)
- [[bench/replay/hardware.py/Hardware/to_dict]] (calls)
- [[bench/replay/records.py/to_jsonl_lines]] (calls)
- [[bench/replay/report.py/build_summary]] (calls)
- [[bench/replay/report.py/render_markdown]] (calls)
- [[bench/replay/session.py/DriftCurve/to_dict]] (calls)
- [[bench/results/flask/config.json/_file]] (references)
- [[bench/results/flask/drift.json/_file]] (references)
- [[bench/results/flask/summary.json/_file]] (references)
- [[bench/results/flask/summary.md/_file]] (references)
- [[bench/results/httpie/config.json/_file]] (references)
- [[bench/results/httpie/drift.json/_file]] (references)
- [[bench/results/httpie/summary.json/_file]] (references)
- [[bench/results/httpie/summary.md/_file]] (references)
- [[bench/results/i508/flask/config.json/_file]] (references)
- [[bench/results/i508/flask/summary.json/_file]] (references)
- [[bench/results/i508/flask/summary.md/_file]] (references)
- [[bench/results/n4/flask/config.json/_file]] (references)
- [[bench/results/n4/flask/summary.json/_file]] (references)
- [[bench/results/n4/flask/summary.md/_file]] (references)
- [[bench/results/n4/httpie/config.json/_file]] (references)
- [[bench/results/n4/httpie/summary.json/_file]] (references)
- [[bench/results/n4/httpie/summary.md/_file]] (references)
- [[bench/results/paired/flask/config.json/_file]] (references)
- [[bench/results/paired/flask/summary.json/_file]] (references)
- [[bench/results/paired/flask/summary.md/_file]] (references)
- [[bench/results/sqlfluff/config.json/_file]] (references)
- [[bench/results/sqlfluff/drift.json/_file]] (references)
- [[bench/results/sqlfluff/summary.json/_file]] (references)
- [[bench/results/sqlfluff/summary.md/_file]] (references)
- [[bench/results/swebench/config.json/_file]] (references)
