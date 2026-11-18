---
id: "bench/replay/covread.py::read_coverage"
source: "bench/replay/covread.py"
start: 115
end: 147
kind: "func"
is-test: false
tags: [rtdd/code]
---
# read_coverage

#rtdd/code

## Tests

- [[bench/tests/test_covread.py/test_a_branch_mode_store_is_read_from_the_arc_table]] (calls)
- [[bench/tests/test_covread.py/test_non_positive_line_numbers_are_never_real_source_lines]] (calls)
- [[bench/tests/test_covread.py/test_the_phases_of_one_test_are_unioned_into_a_single_entry]] (calls)
- [[bench/tests/test_covread.py/test_lines_measured_outside_the_repo_are_dropped]] (calls)
- [[bench/tests/test_covread.py/test_a_store_with_no_has_arcs_row_is_read_as_line_mode]] (calls)
- [[bench/tests/test_falsesignal.py/test_false_signal_rate_against_the_synthetic_repo]] (calls)
- [[bench/tests/test_falsesignal.py/test_truth_is_only_the_instrumented_run_never_the_reported_set]] (calls)

## Callers

- [[bench/tests/test_covread.py/_instrumented_truth]] (calls)

## Callees

- [[bench/replay/covread.py/CoverageTruth]] (calls)
- [[bench/replay/covread.py/normalize_context]] (calls)
- [[bench/replay/covread.py/_rel]] (calls)
- [[bench/replay/covread.py/_rows]] (calls)
