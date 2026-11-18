---
id: "bench/replay/runner.py::run_full"
source: "bench/replay/runner.py"
start: 283
end: 291
kind: "func"
is-test: false
tags: [rtdd/code]
---
# run_full

#rtdd/code

## Tests

- [[bench/tests/test_falsesignal.py/test_false_signal_rate_against_the_synthetic_repo]] (calls)
- [[bench/tests/test_falsesignal.py/test_truth_is_only_the_instrumented_run_never_the_reported_set]] (calls)
- [[bench/tests/test_runner.py/test_run_full_at_green_and_red_commits]] (calls)
- [[bench/tests/test_runner.py/test_instrumented_run_produces_a_coverage_db]] (calls)
- [[bench/tests/test_runner.py/test_every_mode_reports_per_test_durations_and_a_wall_clock]] (calls)
- [[bench/tests/test_runner.py/test_a_collection_error_in_a_real_repo_is_recorded_not_a_crash]] (calls)
- [[bench/tests/test_runner.py/test_an_instrumented_run_that_lost_its_contexts_is_fatal]] (calls)
- [[bench/tests/test_runner.py/test_the_same_warning_on_an_uninstrumented_run_is_not_fatal]] (calls)
- [[bench/tests/test_runner.py/test_an_instrumented_run_forces_ctrace]] (calls)
- [[bench/tests/test_runner.py/test_a_cached_run_of_the_real_suite_skips_the_second_pytest]] (calls)
- [[bench/tests/test_runner.py/test_cacheprovider_flag_populates_pytest_cache]] (calls)
- [[bench/tests/test_runner.py/test_the_cacheprovider_is_disabled_by_default]] (calls)
- [[bench/tests/test_runner.py/test_a_full_runs_exec_args_reach_the_pytest_invocation]] (calls)

## Callers

- [[bench/replay/replay.py/_cached_full]] (calls)
- [[bench/replay/strategies/lastfailed.py/LastFailed/prepare]] (calls)
- [[bench/tests/test_covread.py/_instrumented_truth]] (calls)
- [[bench/tests/test_metrics.py/_records_from_synth]] (calls)

## Callees

- [[bench/replay/runner.py/_invoke]] (calls)
