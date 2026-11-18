---
id: "bench/replay/cli.py::_rebuild_repo"
source: "bench/replay/cli.py"
start: 357
end: 412
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _rebuild_repo

#rtdd/code

## Tests

none

## Callers

- [[bench/replay/cli.py/cmd_rebuild]] (calls)

## Callees

- [[bench/replay/config.py/canonical_json]] (calls)
- [[bench/replay/config.py/RunConfig/from_dict]] (calls)
- [[bench/replay/hardware.py/Hardware/from_dict]] (calls)
- [[bench/replay/records.py/parse_jsonl_lines]] (calls)
- [[bench/replay/replay.py/ReplayOutput]] (calls)
- [[bench/replay/replay.py/strategy_order]] (calls)
- [[bench/replay/report.py/build_summary]] (calls)
- [[bench/replay/report.py/render_markdown]] (calls)
- [[bench/replay/strategies/base.py/get]] (calls)
- [[bench/results/flask/config.json/_file]] (references)
- [[bench/results/flask/summary.json/_file]] (references)
- [[bench/results/flask/summary.md/_file]] (references)
- [[bench/results/httpie/config.json/_file]] (references)
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
- [[bench/results/sqlfluff/summary.json/_file]] (references)
- [[bench/results/sqlfluff/summary.md/_file]] (references)
- [[bench/results/swebench/config.json/_file]] (references)
