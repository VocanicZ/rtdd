---
id: "bench/replay/strategies/rtdd.py::Rtdd"
source: "bench/replay/strategies/rtdd.py"
start: 28
end: 57
kind: "class"
is-test: false
tags: [rtdd/code]
---
# Rtdd

#rtdd/code

## Tests

- [[bench/tests/test_strategy_rounds.py/test_the_round_1_arm_selects_round_1_only]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_prepare_builds_the_graph_with_the_configured_binary]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_select_takes_round_1_expanded_to_collected_ids]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_select_passes_only_shipped_defaults]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_an_empty_round_1_is_an_empty_selection_not_an_escalation]] (calls)
- [[bench/tests/test_strategy_rtdd.py/test_verified_against_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/replay/strategies/rtdd.py/RtddRounds12]] (inherits)

## Callees

- [[bench/replay/strategies/rtdd.py/Rtdd/__init__]] (method)
- [[bench/replay/strategies/rtdd.py/Rtdd/prepare]] (method)
- [[bench/replay/strategies/rtdd.py/Rtdd/_graph_ids]] (method)
- [[bench/replay/strategies/rtdd.py/Rtdd/select]] (method)
