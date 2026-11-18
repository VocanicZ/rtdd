---
id: "bench/replay/runner.py::run_subset"
source: "bench/replay/runner.py"
start: 294
end: 305
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run_subset

#rtdd/code

## Tests

- [[bench/tests/test_runner.py/test_run_subset_runs_only_what_it_is_given]] (calls)
- [[bench/tests/test_runner.py/test_run_subset_of_nothing_never_invokes_pytest]] (calls)
- [[bench/tests/test_runner.py/test_every_mode_reports_per_test_durations_and_a_wall_clock]] (calls)
- [[bench/tests/test_runner.py/test_a_subsets_exec_args_reach_the_pytest_invocation]] (calls)
- [[bench/tests/test_runner.py/test_a_subset_without_exec_args_stays_serial]] (calls)
- [[bench/tests/test_strategy_xdist.py/test_the_xdist_selections_exec_args_reach_the_pytest_invocation]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/runner.py/RunResult]] (calls)
- [[bench/replay/runner.py/_invoke]] (calls)
