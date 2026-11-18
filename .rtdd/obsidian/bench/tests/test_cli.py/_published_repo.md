---
id: "bench/tests/test_cli.py::_published_repo"
source: "bench/tests/test_cli.py"
start: 681
end: 718
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _published_repo

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_report_rebuilds_a_summary_from_the_committed_records]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_leaves_the_run_config_that_stamped_the_result_untouched]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_does_not_touch_a_repo_the_corpus_no_longer_admits]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_keeps_an_operator_wallclock_refusal_refused]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_preserves_what_the_records_do_not_carry]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_publishes_strategies_in_the_order_the_run_did]] (calls)
- [[bench/tests/test_cli.py/test_derive_refuses_a_repo_whose_records_cannot_support_it]] (calls)

## Callers

- [[bench/tests/test_cli.py/_published_with_importgraph]] (calls)

## Callees

- [[bench/replay/config.py/RunConfig]] (calls)
- [[bench/replay/config.py/RunConfig/to_dict]] (calls)
- [[bench/replay/config.py/RunConfig/digest]] (calls)
- [[bench/replay/hardware.py/Hardware]] (calls)
- [[bench/replay/hardware.py/Hardware/to_dict]] (calls)
- [[bench/replay/records.py/CommitRecord]] (calls)
- [[bench/replay/records.py/StrategyRecord]] (calls)
- [[bench/replay/records.py/WallClockRecord]] (calls)
- [[bench/replay/records.py/to_jsonl_lines]] (calls)
- [[bench/results/flask/config.json/_file]] (references)
- [[bench/results/httpie/config.json/_file]] (references)
- [[bench/results/i508/flask/config.json/_file]] (references)
- [[bench/results/n4/flask/config.json/_file]] (references)
- [[bench/results/n4/httpie/config.json/_file]] (references)
- [[bench/results/paired/flask/config.json/_file]] (references)
- [[bench/results/sqlfluff/config.json/_file]] (references)
- [[bench/results/swebench/config.json/_file]] (references)
