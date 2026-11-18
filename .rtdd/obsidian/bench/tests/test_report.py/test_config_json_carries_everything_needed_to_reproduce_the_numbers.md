---
id: "bench/tests/test_report.py::test_config_json_carries_everything_needed_to_reproduce_the_numbers"
source: "bench/tests/test_report.py"
start: 274
end: 284
kind: "func"
is-test: true
tags: [rtdd/test]
---
# test_config_json_carries_everything_needed_to_reproduce_the_numbers

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[bench/replay/config.py/RunConfig/digest]] (calls)
- [[bench/replay/hardware.py/Hardware/fingerprint]] (calls)
- [[bench/replay/report.py/write_results]] (calls)
- [[bench/results/flask/config.json/_file]] (references)
- [[bench/results/httpie/config.json/_file]] (references)
- [[bench/results/i508/flask/config.json/_file]] (references)
- [[bench/results/n4/flask/config.json/_file]] (references)
- [[bench/results/n4/httpie/config.json/_file]] (references)
- [[bench/results/paired/flask/config.json/_file]] (references)
- [[bench/results/sqlfluff/config.json/_file]] (references)
- [[bench/results/swebench/config.json/_file]] (references)
- [[bench/tests/test_report.py/_out]] (calls)
