---
id: "bench/swebench/analyze.py::kill_criterion_met"
source: "bench/swebench/analyze.py"
start: 205
end: 232
kind: "func"
is-test: false
tags: [rtdd/code]
---
# kill_criterion_met

#rtdd/code

## Tests

- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_reads_the_floor_from_the_signed_preregistration]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_is_not_met_when_any_repo_falls_below_the_floor]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_refuses_a_floor_that_disagrees_with_the_signed_file]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_accepts_a_floor_that_matches_the_signed_file]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_refuses_an_unsigned_preregistration]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_refuses_when_m3_has_not_been_published]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_ignores_axis_1s_own_results_directory]] (calls)
- [[bench/swebench/tests/test_analyze.py/test_kill_criterion_refuses_when_no_repo_reports_the_single_test_stratum]] (calls)

## Callers

- [[bench/swebench/analyze.py/analyze]] (calls)

## Callees

- [[bench/PREREGISTRATION.md/_file]] (references)
- [[bench/swebench/analyze.py/_axis2_stratum_recalls]] (calls)
- [[bench/swebench/prereg.py/PreregError]] (calls)
- [[bench/swebench/prereg.py/load]] (calls)
