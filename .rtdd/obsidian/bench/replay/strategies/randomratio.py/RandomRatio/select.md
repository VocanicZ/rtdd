---
id: "bench/replay/strategies/randomratio.py::RandomRatio::select"
source: "bench/replay/strategies/randomratio.py"
start: 47
end: 62
kind: "method"
is-test: false
tags: [rtdd/code]
---
# select

#rtdd/code

## Tests

- [[bench/tests/test_strategy_randomratio.py/test_matches_the_peer_selection_size]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_is_deterministic_for_a_given_seed_and_commit]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_a_different_seed_gives_a_different_sample_at_the_same_commit]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_the_two_variants_of_one_commit_are_sampled_independently]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_missing_peer_size_is_a_harness_error]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_a_peer_that_selected_more_than_the_suite_holds_is_clamped]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_a_peer_that_selected_nothing_samples_nothing]] (calls)
- [[bench/tests/test_strategy_randomratio.py/test_it_samples_real_ids_from_the_synthetic_repo]] (calls)

## Callers

- [[bench/replay/strategies/randomratio.py/RandomRatio]] (method)

## Callees

- [[bench/replay/strategies/base.py/Selection]] (calls)
- [[bench/replay/strategies/randomratio.py/seeded_rng]] (calls)
