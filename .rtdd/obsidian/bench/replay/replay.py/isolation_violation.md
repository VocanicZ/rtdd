---
id: "bench/replay/replay.py::isolation_violation"
source: "bench/replay/replay.py"
start: 173
end: 187
kind: "func"
is-test: false
tags: [rtdd/code]
---
# isolation_violation

#rtdd/code

## Tests

- [[bench/tests/test_replay.py/test_a_subset_that_reproduces_the_expected_failures_is_not_a_violation]] (calls)
- [[bench/tests/test_replay.py/test_a_subset_that_hides_a_failure_it_selected_is_a_violation]] (calls)
- [[bench/tests/test_replay.py/test_a_pre_existing_failure_inside_the_subset_is_not_an_isolation_violation]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

none
