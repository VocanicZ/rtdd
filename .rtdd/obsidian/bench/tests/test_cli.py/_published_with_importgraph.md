---
id: "bench/tests/test_cli.py::_published_with_importgraph"
source: "bench/tests/test_cli.py"
start: 842
end: 850
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _published_with_importgraph

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_derive_appends_a_static_record_per_commit]] (calls)
- [[bench/tests/test_cli.py/test_derive_is_idempotent_to_the_byte]] (calls)
- [[bench/tests/test_cli.py/test_derive_supersedes_a_stale_derived_record_rather_than_doubling_it]] (calls)
- [[bench/tests/test_cli.py/test_derive_leaves_the_measured_records_exactly_as_the_run_wrote_them]] (calls)
- [[bench/tests/test_cli.py/test_derive_never_touches_a_repo_the_corpus_no_longer_admits]] (calls)
- [[bench/tests/test_cli.py/test_derive_leaves_config_json_untouched]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_publishes_an_arm_the_records_carry_and_the_config_does_not]] (calls)
- [[bench/tests/test_cli.py/test_rebuild_orders_a_derived_arm_the_way_strategy_order_does]] (calls)
- [[bench/tests/test_cli.py/test_the_rebuilt_markdown_publishes_the_derived_arm_as_not_measured]] (calls)

## Callers

- [[bench/tests/test_results_static_arm.py/_synth_results]] (calls)

## Callees

- [[bench/replay/records.py/StrategyRecord]] (calls)
- [[bench/replay/records.py/to_jsonl_lines]] (calls)
- [[bench/replay/records.py/parse_jsonl_lines]] (calls)
- [[bench/tests/test_cli.py/_published_repo]] (calls)
- [[internal/scan/testdata/python/calc.py/Calc/plus]] (calls)
