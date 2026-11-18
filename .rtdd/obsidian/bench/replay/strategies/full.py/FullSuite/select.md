---
id: "bench/replay/strategies/full.py::FullSuite::select"
source: "bench/replay/strategies/full.py"
start: 23
end: 30
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_full.py/test_full_selects_everything_and_counts_as_an_escalation]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_preserves_the_collected_order_and_selects_only_known_ids]] (calls)
- [[bench/tests/test_strategy_full.py/test_empty_suite_still_escalates]] (calls)
- [[bench/tests/test_strategy_full.py/test_the_changed_set_does_not_narrow_the_control_arm]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_passes_no_tuning_flags]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)

## Callers

- [[bench/replay/strategies/full.py/FullSuite]] (method)

## Callees

- [[bench/replay/strategies/base.py/Selection]] (calls)
