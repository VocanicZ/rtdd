---
id: "bench/tests/test_runner.py::test_a_config_change_makes_the_run_a_miss_not_a_stale_hit::build"
source: "bench/tests/test_runner.py"
start: 351
end: 352
kind: "func"
is-test: false
tags: [rtdd/code]
---
# build

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_a_config_change_makes_the_run_a_miss_not_a_stale_hit]] (calls)

## Callers

- [[bench/replay/cache.py/Cache/json_or_build]] (calls)
- [[bench/replay/replay.py/_cached_selection/build_dict]] (calls)
- [[bench/replay/runner.py/cached_run]] (calls)
- [[bench/replay/session.py/run_drift]] (calls)

## Callees

- [[bench/replay/runner.py/RunResult]] (calls)
