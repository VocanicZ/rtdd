---
id: "bench/replay/strategies/rtdd.py::Rtdd::select"
source: "bench/replay/strategies/rtdd.py"
start: 44
end: 57
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_rounds.py/test_the_round_1_arm_selects_round_1_only]] (calls)
- [[bench/tests/test_strategy_rounds.py/test_the_rounds_1_2_arm_is_the_union_without_double_counting]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_select_takes_round_1_expanded_to_collected_ids]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_select_passes_only_shipped_defaults]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_an_empty_round_1_is_an_empty_selection_not_an_escalation]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_verified_against_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/strategies/rtdd.py/Rtdd]] (method)

## Callees

- [[bench/replay/graphids.py/expand_graph_ids]] (calls)
- [[bench/replay/rtddio.py/which]] (calls)
- [[bench/replay/strategies/base.py/Selection]] (calls)
- [[bench/replay/strategies/rtdd.py/Rtdd/_graph_ids]] (calls)
