---
id: "bench/tests/synthrepo.py::build_synth_repo"
source: "bench/tests/synthrepo.py"
start: 94
end: 139
kind: "func"
is-test: false
tags: [rtdd/code]
---
# build_synth_repo

#rtdd/code

## Tests

- [[bench/tests/synthrepo.py/_file_]] (calls)
- [[bench/tests/test_synthrepo.py/test_history_shape_and_real_outcomes]] (calls)
- [[bench/tests/test_synthrepo.py/test_full_suite_outcomes_match_the_hand_computed_table]] (calls)
- [[bench/tests/test_synthrepo.py/test_f_full_against_the_parent_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_synthrepo.py/test_a_failure_already_present_in_the_parent_is_not_counted_as_f_full]] (calls)
- [[bench/tests/test_synthrepo.py/test_rebuilding_the_fixture_is_deterministic]] (calls)

## Callers

- [[bench/tests/conftest.py/synth]] (calls)

## Callees

- [[bench/replay/__init__.py/_file]] (references)
- [[bench/replay/strategies/__init__.py/_file]] (references)
- [[bench/swebench/__init__.py/_file]] (references)
- [[bench/swebench/budget.py/Budget/add]] (calls)
- [[bench/swebench/providers/__init__.py/_file]] (references)
- [[bench/swebench/tests/test_agent.py/_code_tokens/add]] (calls)
- [[bench/tests/synthrepo.py/SynthRepo]] (calls)
- [[bench/tests/synthrepo.py/_git]] (calls)
- [[bench/tests/synthrepo.py/_write]] (calls)
- [[bench/tests/synthrepo.py/_commit]] (calls)
- [[cmd/rtdd/testdata/which-golden/edit/src/calc.py/add]] (calls)
- [[cmd/rtdd/testdata/which-golden/repo/src/calc.py/add]] (calls)
- [[cmd/rtdd/testdata/which-golden/repo/tests/test_calc.py/test_add]] (calls)
- [[internal/scan/testdata/python/calc.py/add]] (calls)
- [[internal/scan/testdata/python/test_calc.py/test_add]] (calls)
