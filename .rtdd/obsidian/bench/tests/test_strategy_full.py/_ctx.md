---
id: "bench/tests/test_strategy_full.py::_ctx"
source: "bench/tests/test_strategy_full.py"
start: 19
end: 31
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _ctx

#rtdd/code

## Tests

- [[bench/tests/test_strategy_full.py/test_full_selects_everything_and_counts_as_an_escalation]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_preserves_the_collected_order_and_selects_only_known_ids]] (calls)
- [[bench/tests/test_strategy_full.py/test_empty_suite_still_escalates]] (calls)
- [[bench/tests/test_strategy_full.py/test_the_changed_set_does_not_narrow_the_control_arm]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_passes_no_tuning_flags]] (calls)

## Callers

none

## Callees

- [[bench/replay/gitwork.py/Change]] (calls)
- [[bench/replay/strategies/base.py/CommitContext]] (calls)
